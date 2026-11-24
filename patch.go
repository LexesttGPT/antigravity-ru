package main

import (
	"bytes"
	"crypto/sha256"
	"debug/pe"
	_ "embed"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

//go:embed translations.json
var dictionary []byte

//go:embed localize.js
var localizer []byte

const originalHash = "9f0aad7274795d7fbde04931d96b348dcfc5f3d81d7373e6b70aeb09307442df"
const originalExe = "fdf8a0a6af474d54301740bdf0144d95a30be3105d96a2cb39bb8086278513ac"
const preloadHash = "2973a6bfd3154fd20c58cbb6643dbf0a5e0cc4fd8a5894c4496f3db5c6231475"
const mainHash = "e4dd0d71d96f7fdcb5af5b86395cebac8d2367812604fa331429570a442fed69"
const targetName = "dist/preload.js"
const legacyBackup = "app.asar.ru-alpha.original"

var oldHashes = map[string]bool{"f5a17d2b69ffbc7fbfbe6487462b14e505f6045e5d6b0a00a2bd6326eb4fa37d": true, "0b96986427e8c8242cdea6ffae416a569e549f777af7e02f2ac65abf03196c0a": true, "dcdda6b311cd9f4c1a5111a0efce3a7beaa20dc3342f070b741b192ee47e5966": true, "95d4fa88d0237f4b8a47935610bd0a0bc5f94fa6cec2197a55d218a7785f47de": true, "b953992a4df966308c380746f168ef3d54308779e81488fe3780b8ce98c2c817": true}

const marker = "/* ANTIGRAVITY_RU_OFFLINE_0_6_0 */"
const maxArchive = 128 << 20

var versionRE = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][a-zA-Z0-9.-]+)?$`)
var hashRE = regexp.MustCompile(`^[a-f0-9]{64}$`)

type obj = map[string]any
type archive struct {
	header  obj
	payload []byte
	entries map[string]obj
}
type span struct {
	off, end int
	name     string
}

func hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func hashFile(p string) (string, error) {
	f, e := os.Open(p)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	_, e = io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), e
}
func readRegular(p string, max int64) ([]byte, error) {
	st, e := os.Lstat(p)
	if e != nil {
		return nil, e
	}
	if !st.Mode().IsRegular() || st.Size() > max {
		return nil, fmt.Errorf("недопустимый файл: %s", p)
	}
	return os.ReadFile(p)
}
func asObj(x any) (obj, error) {
	v, ok := x.(map[string]any)
	if !ok {
		return nil, errors.New("неверные метаданные ASAR")
	}
	return v, nil
}
func num(x any) (int, error) {
	v, ok := x.(json.Number)
	if !ok {
		return 0, errors.New("неверное число ASAR")
	}
	n, e := strconv.ParseInt(string(v), 10, 32)
	if e != nil || n < 0 {
		return 0, errors.New("неверное число ASAR")
	}
	return int(n), nil
}
func offset(x any) (int, error) {
	s, ok := x.(string)
	if !ok {
		return 0, errors.New("неверное смещение")
	}
	n, e := strconv.ParseInt(s, 10, 32)
	if e != nil || n < 0 {
		return 0, errors.New("неверное смещение")
	}
	return int(n), nil
}
func parse(raw []byte) (*archive, error) {
	if len(raw) < 16 || len(raw) > maxArchive {
		return nil, errors.New("неверный размер ASAR")
	}
	a, b, c, d := binary.LittleEndian.Uint32(raw), binary.LittleEndian.Uint32(raw[4:]), binary.LittleEndian.Uint32(raw[8:]), binary.LittleEndian.Uint32(raw[12:])
	if a != 4 || b < 8 || c < 4 || b != c+4 || d > c-4 || uint64(b)+8 > uint64(len(raw)) || uint64(d)+16 > uint64(len(raw)) {
		return nil, errors.New("неверный заголовок ASAR")
	}
	dec := json.NewDecoder(bytes.NewReader(raw[16 : 16+int(d)]))
	dec.UseNumber()
	var h obj
	if e := dec.Decode(&h); e != nil {
		return nil, e
	}
	if len(h) != 1 {
		return nil, errors.New("неизвестная структура ASAR")
	}
	ar := &archive{h, raw[8+int(b):], map[string]obj{}}
	var spans []span
	var walk func(obj, string, int) error
	walk = func(node obj, prefix string, depth int) error {
		if depth > 64 {
			return errors.New("глубина ASAR")
		}
		fs, e := asObj(node["files"])
		if e != nil {
			return e
		}
		for k, v := range fs {
			if k == "" || k == "." || k == ".." || strings.ContainsAny(k, "/\\\x00") {
				return errors.New("небезопасное имя ASAR")
			}
			m, e := asObj(v)
			if e != nil {
				return e
			}
			name := prefix + k
			if _, ok := m["files"]; ok {
				if e = walk(m, name+"/", depth+1); e != nil {
					return e
				}
				continue
			}
			ar.entries[name] = m
			n, e := num(m["size"])
			if e != nil {
				return e
			}
			if m["unpacked"] == true {
				if _, ok := m["offset"]; ok {
					return errors.New("неизвестный unpacked ASAR")
				}
				continue
			}
			o, e := offset(m["offset"])
			if e != nil || o > len(ar.payload) || n > len(ar.payload)-o {
				return errors.New("неверный диапазон ASAR")
			}
			if e = verifyIntegrity(m, ar.payload[o:o+n]); e != nil {
				return fmt.Errorf("%s: %w", name, e)
			}
			if n > 0 {
				spans = append(spans, span{o, o + n, name})
			}
		}
		return nil
	}
	if e := walk(h, "", 0); e != nil {
		return nil, e
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].off < spans[j].off })
	last := 0
	for _, s := range spans {
		if s.off != last {
			return nil, errors.New("неизвестная компоновка ASAR")
		}
		last = s.end
	}
	if last != len(ar.payload) {
		return nil, errors.New("лишние данные ASAR")
	}
	return ar, nil
}
func verifyIntegrity(m obj, b []byte) error {
	in, e := asObj(m["integrity"])
	if e != nil {
		return e
	}
	if in["algorithm"] != "SHA256" || in["hash"] != hash(b) {
		return errors.New("ошибка SHA-256")
	}
	block, e := num(in["blockSize"])
	if e != nil || block < 1 || block > maxArchive {
		return errors.New("неверный размер блока")
	}
	ls, ok := in["blocks"].([]any)
	if !ok {
		return errors.New("нет хешей блоков")
	}
	count := (len(b) + block - 1) / block
	if len(b) == 0 && len(ls) == 1 && ls[0] == hash(nil) {
		return nil
	}
	if count != len(ls) {
		return errors.New("неверное число блоков")
	}
	for i := 0; i < count; i++ {
		end := min(len(b), (i+1)*block)
		if ls[i] != hash(b[i*block:end]) {
			return errors.New("ошибка блока SHA-256")
		}
	}
	return nil
}
func (a *archive) member(k string) ([]byte, error) {
	v, ok := a.entries[k]
	if !ok {
		return nil, fmt.Errorf("нет %s", k)
	}
	o, e := offset(v["offset"])
	if e != nil {
		return nil, e
	}
	n, e := num(v["size"])
	if e != nil {
		return nil, e
	}
	return a.payload[o : o+n], nil
}
func compatible(a *archive) (string, error) {
	p, e := a.member("package.json")
	if e != nil {
		return "", e
	}
	var pkg struct {
		Name, ProductName, Version, Main string
		Author                           struct{ Name string }
	}
	if e = json.Unmarshal(p, &pkg); e != nil {
		return "", e
	}
	if pkg.Name != "antigravity" || pkg.ProductName != "Antigravity" || pkg.Main != "dist/main.js" || pkg.Author.Name != "Google" || !versionRE.MatchString(pkg.Version) {
		return "", errors.New("неизвестный пакет приложения")
	}
	pre, e := a.member(targetName)
	if e != nil {
		return "", e
	}
	main, e := a.member("dist/main.js")
	if e != nil {
		return "", e
	}
	if hash(pre) != preloadHash || hash(main) != mainHash {
		return "", errors.New("изменилась архитектура загрузчика: требуется новый аудит. Ничего не изменено")
	}
	return pkg.Version, nil
}
func encode(a *archive, payload []byte) ([]byte, error) {
	j, e := json.Marshal(a.header)
	if e != nil {
		return nil, e
	}
	pad := (4 - len(j)%4) % 4
	inner := 8 + len(j) + pad
	r := make([]byte, 8+inner, 8+inner+len(payload))
	binary.LittleEndian.PutUint32(r, 4)
	binary.LittleEndian.PutUint32(r[4:], uint32(inner))
	binary.LittleEndian.PutUint32(r[8:], uint32(inner-4))
	binary.LittleEndian.PutUint32(r[12:], uint32(len(j)))
	copy(r[16:], j)
	return append(r, payload...), nil
}
func makePatch(raw []byte) ([]byte, string, error) {
	a, e := parse(raw)
	if e != nil {
		return nil, "", e
	}
	ver, e := compatible(a)
	if e != nil {
		return nil, "", e
	}
	old, _ := a.member(targetName)
	m := a.entries[targetName]
	off, _ := offset(m["offset"])
	n := len(old)
	if bytes.Count(localizer, []byte("__DICT__")) != 1 || !json.Valid(dictionary) {
		return nil, "", errors.New("неверный встроенный словарь")
	}
	script := bytes.Replace(localizer, []byte("__DICT__"), dictionary, 1)
	newb := append(append(append([]byte{}, old...), []byte("\n"+marker+"\n")...), script...)
	delta := len(newb) - n
	m["size"] = json.Number(strconv.Itoa(len(newb)))
	integ, _ := asObj(m["integrity"])
	block, _ := num(integ["blockSize"])
	integ["hash"] = hash(newb)
	blocks := []any{}
	for i := 0; i < len(newb); i += block {
		blocks = append(blocks, hash(newb[i:min(len(newb), i+block)]))
	}
	integ["blocks"] = blocks
	for k, x := range a.entries {
		if k == targetName || x["unpacked"] == true {
			continue
		}
		o, _ := offset(x["offset"])
		if o > off {
			x["offset"] = strconv.Itoa(o + delta)
		}
	}
	payload := make([]byte, 0, len(a.payload)+delta)
	payload = append(payload, a.payload[:off]...)
	payload = append(payload, newb...)
	payload = append(payload, a.payload[off+n:]...)
	patched, e := encode(a, payload)
	if e != nil {
		return nil, "", e
	}
	if e = verifyPatch(raw, patched, newb); e != nil {
		return nil, "", e
	}
	return patched, ver, nil
}
func verifyPatch(raw, patched, expected []byte) error {
	a, e := parse(raw)
	if e != nil {
		return e
	}
	b, e := parse(patched)
	if e != nil {
		return e
	}
	if len(a.entries) != len(b.entries) {
		return errors.New("изменился список файлов")
	}
	for k, m := range a.entries {
		q, ok := b.entries[k]
		if !ok {
			return errors.New("изменился список файлов")
		}
		if k == targetName {
			v, e := b.member(k)
			if e != nil || !bytes.Equal(v, expected) {
				return errors.New("неверный патч")
			}
			continue
		}
		if m["unpacked"] != true {
			v, e := a.member(k)
			if e != nil {
				return e
			}
			w, e := b.member(k)
			if e != nil || !bytes.Equal(v, w) {
				return fmt.Errorf("изменён другой файл: %s", k)
			}
		}
		delete(m, "offset")
		delete(q, "offset")
		x, _ := json.Marshal(m)
		y, _ := json.Marshal(q)
		if !bytes.Equal(x, y) {
			return fmt.Errorf("изменены метаданные: %s", k)
		}
	}
	return nil
}

type manifest struct{ Version, Original, Patched, Exe string }

func backupPath(dir, h string) string   { return filepath.Join(dir, "app.asar.ru-backup-"+h+".original") }
func manifestPath(dir, h string) string { return filepath.Join(dir, "app.asar.ru-state-"+h+".json") }
func createExclusive(p string, b []byte) error {
	f, e := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	ok := false
	defer func() {
		if !ok {
			os.Remove(p)
		}
	}()
	if _, e = f.Write(b); e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	check, e := readRegular(p, maxArchive)
	if e != nil || !bytes.Equal(check, b) {
		return errors.New("ошибка проверки записи")
	}
	ok = true
	return nil
}
func ensureFile(p string, b []byte) error {
	old, e := readRegular(p, maxArchive)
	if e == nil {
		if bytes.Equal(old, b) {
			return nil
		}
		return fmt.Errorf("существующая копия отличается: %s", p)
	}
	if !os.IsNotExist(e) {
		return e
	}
	return createExclusive(p, b)
}
func atomicWrite(p string, current, next []byte) error {
	f, e := os.CreateTemp(filepath.Dir(p), ".ag-ru-*.tmp")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, e = f.Write(next); e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	check, e := readRegular(tmp, maxArchive)
	if e != nil || !bytes.Equal(check, next) {
		return errors.New("ошибка проверки временного файла")
	}
	old, e := readRegular(p, maxArchive)
	if e != nil || !bytes.Equal(old, current) {
		return errors.New("файл изменился во время операции")
	}
	if e = os.Rename(tmp, p); e != nil {
		return e
	}
	out, e := readRegular(p, maxArchive)
	if e != nil || !bytes.Equal(out, next) {
		return errors.New("не удалось подтвердить запись; резервная копия сохранена")
	}
	return nil
}
func resolveOriginal(dir string, current []byte, exeHash string) ([]byte, error) {
	h := hash(current)
	if h == originalHash && exeHash != originalExe {
		return nil, errors.New("EXE не совпадает с проверенной сборкой 2.35.1")
	}
	if oldHashes[h] {
		if exeHash != originalExe {
			return nil, errors.New("старая русификация не соответствует EXE")
		}
		b, e := readRegular(filepath.Join(dir, legacyBackup), maxArchive)
		if e != nil || hash(b) != originalHash {
			return nil, errors.New("отсутствует проверенная оригинальная копия Alpha")
		}
		return b, nil
	}
	mp := manifestPath(dir, h)
	if b, e := readRegular(mp, 1<<20); e == nil {
		var m manifest
		if json.Unmarshal(b, &m) != nil || m.Patched != h || m.Exe != exeHash || !hashRE.MatchString(m.Original) {
			return nil, errors.New("неверная запись о русификации")
		}
		orig, e := readRegular(backupPath(dir, m.Original), maxArchive)
		if e != nil || hash(orig) != m.Original {
			return nil, errors.New("резервная копия повреждена")
		}
		expected, v, e := makePatch(orig)
		if e != nil || v != m.Version || !bytes.Equal(expected, current) {
			return nil, errors.New("патч не соответствует резервной копии")
		}
		return orig, nil
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	return current, nil
}
func perform(root, action string, stopped func() error) (string, error) {
	if action != "check" && action != "install" && action != "uninstall" {
		return "", errors.New("неизвестное действие")
	}
	root, e := filepath.Abs(root)
	if e != nil {
		return "", e
	}
	dir := filepath.Join(root, "resources")
	for _, p := range []string{root, dir} {
		st, e := os.Lstat(p)
		if e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("неверная папка: %s", p)
		}
	}
	if action != "check" {
		if e = stopped(); e != nil {
			return "", e
		}
		// An exclusive lock prevents competing instances of this patcher.
		lp := filepath.Join(dir, ".antigravity-ru.lock")
		lock, err := os.OpenFile(lp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return "", fmt.Errorf("не удалось получить блокировку русификатора: %w", err)
		}
		_, _ = fmt.Fprintln(lock, os.Getpid())
		defer func() { lock.Close(); os.Remove(lp) }()
	}
	exe := filepath.Join(root, "Antigravity.exe")
	st, e := os.Lstat(exe)
	if e != nil || !st.Mode().IsRegular() {
		return "", errors.New("Antigravity.exe не найден")
	}
	eh, e := hashFile(exe)
	if e != nil {
		return "", e
	}
	p := filepath.Join(dir, "app.asar")
	cur, e := readRegular(p, maxArchive)
	if e != nil {
		return "", e
	}
	orig, e := resolveOriginal(dir, cur, eh)
	if e != nil {
		return "", e
	}
	newb, v, e := makePatch(orig)
	if e != nil {
		return "", e
	}
	if v != "2.35.1" {
		f, err := pe.Open(exe)
		if err != nil {
			return "", errors.New("неизвестный EXE: формат PE не подтверждён")
		}
		machine := f.Machine
		f.Close()
		if machine != pe.IMAGE_FILE_MACHINE_AMD64 {
			return "", errors.New("поддерживается только Windows x64")
		}
	}
	if v == "2.35.1" && (hash(orig) != originalHash || eh != originalExe) {
		return "", errors.New("неизвестная сборка 2.35.1")
	}
	msg := fmt.Sprintf("Версия: %s\nОригинал: %s\nТекущий: %s\nПатч: %s", v, hash(orig), hash(cur), hash(newb))
	if v != "2.35.1" {
		msg += "\nНовая версия: совпадают проверенные загрузчики и структура; работа интерфейса не подтверждена."
	}
	if action == "check" {
		return msg + "\nПроверка пройдена. Файлы не изменены.", nil
	}
	if e = stopped(); e != nil {
		return "", e
	}
	if eh2, e := hashFile(exe); e != nil || eh2 != eh {
		return "", errors.New("EXE изменился во время операции")
	}
	if action == "uninstall" {
		if bytes.Equal(cur, orig) {
			return msg + "\nУже оригинал.", nil
		}
		if e = stopped(); e != nil {
			return "", e
		}
		if e = atomicWrite(p, cur, orig); e != nil {
			return "", e
		}
		return msg + "\nОригинал восстановлен.", nil
	}
	if bytes.Equal(cur, newb) {
		return msg + "\nПеревод уже установлен.", nil
	}
	if e = ensureFile(backupPath(dir, hash(orig)), orig); e != nil {
		return "", e
	}
	m, _ := json.MarshalIndent(manifest{v, hash(orig), hash(newb), eh}, "", "  ")
	if e = ensureFile(manifestPath(dir, hash(newb)), m); e != nil {
		return "", e
	}
	if e = stopped(); e != nil {
		return "", e
	}
	if e = atomicWrite(p, cur, newb); e != nil {
		return "", e
	}
	return msg + "\nРусификация установлена. Резервная копия сохранена.", nil
}
