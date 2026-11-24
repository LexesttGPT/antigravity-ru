package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func fixture(t *testing.T) ([]byte, string) {
	t.Helper()
	p := os.Getenv("AG_TEST_ASAR")
	e := os.Getenv("AG_TEST_EXE")
	if p == "" || e == "" {
		t.Skip("set AG_TEST_ASAR and AG_TEST_EXE to original installer resources")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if hash(b) != originalHash {
		t.Fatal("unexpected fixture")
	}
	return b, e
}
func setup(t *testing.T, b []byte, exe string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "resources")
	if e := os.Mkdir(dir, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.Link(exe, filepath.Join(root, "Antigravity.exe")); e != nil {
		v, e := os.ReadFile(exe)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(root, "Antigravity.exe"), v, 0700); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.WriteFile(filepath.Join(dir, "app.asar"), b, 0600); e != nil {
		t.Fatal(e)
	}
	return root
}
func op(t *testing.T, r, a string) {
	t.Helper()
	msg, e := perform(r, a, func() error { return nil })
	if e != nil {
		t.Fatal(a, e)
	}
	t.Log(msg)
}
func contents(t *testing.T, r string) []byte {
	t.Helper()
	b, e := os.ReadFile(filepath.Join(r, "resources", "app.asar"))
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestOriginalInstallRollback(t *testing.T) {
	orig, exe := fixture(t)
	r := setup(t, orig, exe)
	op(t, r, "check")
	if !bytes.Equal(contents(t, r), orig) {
		t.Fatal("check modified")
	}
	op(t, r, "install")
	patched := contents(t, r)
	if bytes.Equal(patched, orig) {
		t.Fatal("nothing changed")
	}
	op(t, r, "install")
	if !bytes.Equal(contents(t, r), patched) {
		t.Fatal("not idempotent")
	}
	op(t, r, "uninstall")
	if !bytes.Equal(contents(t, r), orig) {
		t.Fatal("rollback mismatch")
	}
	op(t, r, "install")
	if !bytes.Equal(contents(t, r), patched) {
		t.Fatal("reinstall mismatch")
	}
}
func TestLegacyUpgrade(t *testing.T) {
	orig, exe := fixture(t)
	path := os.Getenv("AG_TEST_05")
	if path == "" {
		t.Skip("legacy fixture missing")
	}
	old, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if !oldHashes[hash(old)] {
		t.Fatal("legacy unknown", hash(old))
	}
	r := setup(t, old, exe)
	if e = os.WriteFile(filepath.Join(r, "resources", legacyBackup), orig, 0600); e != nil {
		t.Fatal(e)
	}
	op(t, r, "install")
	op(t, r, "uninstall")
	if !bytes.Equal(contents(t, r), orig) {
		t.Fatal("rollback mismatch")
	}
}
func TestRejectCorruption(t *testing.T) {
	orig, exe := fixture(t)
	orig = append([]byte{}, orig...)
	orig[len(orig)-1] ^= 1
	r := setup(t, orig, exe)
	if _, e := perform(r, "install", func() error { return nil }); e == nil {
		t.Fatal("corrupt accepted")
	}
	if !bytes.Equal(contents(t, r), orig) {
		t.Fatal("modified corrupt")
	}
}
func TestFutureVersionStructural(t *testing.T) {
	orig, exe := fixture(t)
	a, e := parse(orig)
	if e != nil {
		t.Fatal(e)
	}
	old, e := a.member("package.json")
	if e != nil {
		t.Fatal(e)
	}
	replacement := bytes.Replace(old, []byte("2.35.1"), []byte("2.35.2"), 1)
	if len(replacement) != len(old) {
		t.Fatal("length")
	}
	m := a.entries["package.json"]
	off, _ := offset(m["offset"])
	payload := append([]byte{}, a.payload...)
	copy(payload[off:], replacement)
	in, _ := asObj(m["integrity"])
	in["hash"] = hash(replacement)
	in["blocks"] = []any{hash(replacement)}
	raw, e := encode(a, payload)
	if e != nil {
		t.Fatal(e)
	}
	r := setup(t, raw, exe)
	op(t, r, "install")
	op(t, r, "uninstall")
	if !bytes.Equal(contents(t, r), raw) {
		t.Fatal("restored wrong version")
	}
	a, e = parse(raw)
	if e != nil {
		t.Fatal(e)
	}
	m = a.entries[targetName]
	off, _ = offset(m["offset"])
	payload = append([]byte{}, a.payload...)
	payload[off] ^= 1
	n, _ := num(m["size"])
	in, _ = asObj(m["integrity"])
	in["hash"] = hash(payload[off : off+n])
	in["blocks"] = []any{hash(payload[off : off+n])}
	m["size"] = json.Number(strconv.Itoa(n))
	bad, e := encode(a, payload)
	if e != nil {
		t.Fatal(e)
	}
	r = setup(t, bad, exe)
	if _, e = perform(r, "install", func() error { return nil }); e == nil {
		t.Fatal("unknown preload accepted")
	}
	if !bytes.Equal(contents(t, r), bad) {
		t.Fatal("unknown preload changed")
	}
}
func TestStopAndLock(t *testing.T) {
	orig, exe := fixture(t)
	r := setup(t, orig, exe)
	if _, e := perform(r, "install", func() error { return os.ErrPermission }); e == nil {
		t.Fatal("running accepted")
	}
	if !bytes.Equal(contents(t, r), orig) {
		t.Fatal("modified while running")
	}
	lp := filepath.Join(r, "resources", ".antigravity-ru.lock")
	if e := os.WriteFile(lp, []byte("test"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := perform(r, "install", func() error { return nil }); e == nil {
		t.Fatal("lock ignored")
	}
	if !bytes.Equal(contents(t, r), orig) {
		t.Fatal("modified locked file")
	}
	if b, e := os.ReadFile(lp); e != nil || string(b) != "test" {
		t.Fatal("foreign lock modified")
	}
}
func TestBackupTamper(t *testing.T) {
	orig, exe := fixture(t)
	r := setup(t, orig, exe)
	op(t, r, "install")
	cur := contents(t, r)
	if e := os.WriteFile(backupPath(filepath.Join(r, "resources"), hash(orig)), []byte("tampered"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := perform(r, "uninstall", func() error { return nil }); e == nil {
		t.Fatal("tamper accepted")
	}
	if !bytes.Equal(contents(t, r), cur) {
		t.Fatal("target changed")
	}
}
