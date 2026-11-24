//go:build windows

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

var (
	u32                       = syscall.NewLazyDLL("user32.dll")
	k32                       = syscall.NewLazyDLL("kernel32.dll")
	gdi                       = syscall.NewLazyDLL("gdi32.dll")
	shell                     = syscall.NewLazyDLL("shell32.dll")
	ole                       = syscall.NewLazyDLL("ole32.dll")
	register                  = u32.NewProc("RegisterClassExW")
	create                    = u32.NewProc("CreateWindowExW")
	def                       = u32.NewProc("DefWindowProcW")
	show                      = u32.NewProc("ShowWindow")
	update                    = u32.NewProc("UpdateWindow")
	getmsg                    = u32.NewProc("GetMessageW")
	translate                 = u32.NewProc("TranslateMessage")
	dispatch                  = u32.NewProc("DispatchMessageW")
	quit                      = u32.NewProc("PostQuitMessage")
	destroy                   = u32.NewProc("DestroyWindow")
	send                      = u32.NewProc("SendMessageW")
	settext                   = u32.NewProc("SetWindowTextW")
	getlen                    = u32.NewProc("GetWindowTextLengthW")
	gettext                   = u32.NewProc("GetWindowTextW")
	mb                        = u32.NewProc("MessageBoxW")
	loadcursor                = u32.NewProc("LoadCursorW")
	module                    = k32.NewProc("GetModuleHandleW")
	stock                     = gdi.NewProc("GetStockObject")
	browse                    = shell.NewProc("SHBrowseForFolderW")
	getpath                   = shell.NewProc("SHGetPathFromIDListW")
	free                      = ole.NewProc("CoTaskMemFree")
	coinit                    = ole.NewProc("CoInitializeEx")
	couninit                  = ole.NewProc("CoUninitialize")
	hinst, hwnd, edit, logBox uintptr
	font                      uintptr
)

const (
	wmCommand      = 0x111
	wmDestroy      = 2
	wmClose        = 0x10
	wmSetFont      = 0x30
	child          = 0x40000000
	visible        = 0x10000000
	tabstop        = 0x10000
	wsBorder       = 0x00800000
	wsVscroll      = 0x00200000
	wsOverlapped   = 0x00ca0000
	esMultiline    = 4
	esAutoVscroll  = 0x40
	esReadOnly     = 0x800
	esAutoHscroll  = 0x80
	exClientEdge   = 0x200
	idBrowse       = 100
	idCheck        = 101
	idInstall      = 102
	idUndo         = 103
	wmStartupClose = 0x8001
)

type wndClass struct {
	Size, Style                        uint32
	Callback                           uintptr
	ClsExtra, WndExtra                 int32
	Instance, Icon, Cursor, Background uintptr
	MenuName, ClassName                *uint16
	SmallIcon                          uintptr
}
type point struct{ X, Y int32 }
type msg struct {
	Wnd            uintptr
	Message        uint32
	Wparam, Lparam uintptr
	Time           uint32
	Pt             point
	Private        uint32
}
type browseInfo struct {
	Owner, Root        uintptr
	DisplayName, Title *uint16
	Flags              uint32
	Callback, Lparam   uintptr
	Image              int32
}

func u(s string) *uint16       { return syscall.StringToUTF16Ptr(s) }
func ptr(s string) uintptr     { return uintptr(unsafe.Pointer(u(s))) }
func text(h uintptr, s string) { settext.Call(h, ptr(strings.ReplaceAll(s, "\n", "\r\n"))) }
func message(s, title string, flags uintptr) uintptr {
	r, _, _ := mb.Call(hwnd, ptr(s), ptr(title), flags)
	return r
}
func control(class, label string, style, ex uintptr, x, y, w, h int, id uintptr) uintptr {
	r, _, _ := create.Call(ex, ptr(class), ptr(label), style|child|visible, uintptr(x), uintptr(y), uintptr(w), uintptr(h), hwnd, id, hinst, 0)
	send.Call(r, wmSetFont, font, 1)
	return r
}
func rootValue() string {
	n, _, _ := getlen.Call(edit)
	if n == 0 || n > 32767 {
		return ""
	}
	b := make([]uint16, n+1)
	gettext.Call(edit, uintptr(unsafe.Pointer(&b[0])), n+1)
	return strings.TrimSpace(strings.Trim(syscall.UTF16ToString(b), `"`))
}
func locateDefault() string {
	var matches []string
	for _, p := range []string{filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Antigravity"), filepath.Join(os.Getenv("LOCALAPPDATA"), "Antigravity"), filepath.Join(os.Getenv("ProgramFiles"), "Antigravity"), filepath.Join(os.Getenv("ProgramFiles(x86)"), "Antigravity")} {
		if _, e := os.Stat(filepath.Join(p, "Antigravity.exe")); e == nil {
			matches = append(matches, p)
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	return ""
}
func choose() {
	buf := make([]uint16, 260)
	bi := browseInfo{Owner: hwnd, DisplayName: &buf[0], Title: u("Выберите папку с Antigravity.exe"), Flags: 0x41}
	r, _, _ := browse.Call(uintptr(unsafe.Pointer(&bi)))
	if r != 0 {
		defer free.Call(r)
		p := make([]uint16, 260)
		ok, _, _ := getpath.Call(r, uintptr(unsafe.Pointer(&p[0])))
		if ok != 0 {
			text(edit, syscall.UTF16ToString(p))
		}
	}
}
func operation(action string) {
	root := rootValue()
	if root == "" {
		message("Выберите папку Antigravity.", "Папка не указана", 0x30)
		return
	}
	if action != "check" {
		text(logBox, "Проверка файлов…")
		preview, err := perform(root, "check", ensureStopped)
		if err != nil {
			text(logBox, err.Error())
			message(err.Error(), "Antigravity RU — ошибка", 0x10)
			return
		}
		text(logBox, preview)
		what := "Установить русский интерфейс? Будет изменён только app.asar; оригинал сохранится."
		if action == "uninstall" {
			what = "Восстановить оригинальный интерфейс из проверенной копии?"
		}
		if message(what+"\n\n"+strings.SplitN(preview, "\n", 2)[0]+"\nНовая версия может оказаться несовместимой даже при успешной проверке.", "Подтверждение", 0x24) != 6 {
			return
		}
	}
	if action != "check" {
		text(logBox, "Штатное закрытие Antigravity…")
		update.Call(logBox)
		if e := stopForPatch(root); e != nil {
			text(logBox, e.Error())
			message(e.Error(), "Antigravity RU — операция остановлена", 0x30)
			return
		}
	}
	text(logBox, "Проверка файлов…")
	r, e := perform(root, action, ensureStopped)
	if e != nil {
		r = "Операция остановлена.\n" + e.Error()
		text(logBox, r)
		message(r, "Antigravity RU — ошибка", 0x10)
		return
	}
	text(logBox, r)
	if action != "check" {
		message("Операция успешно завершена. Подробности в журнале.", "Antigravity RU", 0x40)
	}
}
func wndProc(w uintptr, m uint32, a, b uintptr) uintptr {
	switch m {
	case wmStartupClose:
		root := rootValue()
		if root != "" {
			text(logBox, "Штатное закрытие Antigravity…")
			update.Call(logBox)
			if e := stopForPatch(root); e != nil {
				text(logBox, e.Error()+"\nПри установке или восстановлении будет повторена проверка.")
			} else {
				text(logBox, "Antigravity не запущен. Готово к проверке и установке.\nНовые сборки принимаются только при совпадении проверенных загрузчиков.")
			}
		}
		return 0
	case wmCommand:
		switch a & 0xffff {
		case idBrowse:
			choose()
		case idCheck:
			operation("check")
		case idInstall:
			operation("install")
		case idUndo:
			operation("uninstall")
		}
		return 0
	case wmClose:
		destroy.Call(w)
		return 0
	case wmDestroy:
		quit.Call(0)
		return 0
	}
	r, _, _ := def.Call(w, uintptr(m), a, b)
	return r
}
func main() {
	runtime.LockOSThread()
	coinit.Call(0, 2)
	defer couninit.Call()
	hinst, _, _ = module.Call(0)
	font, _, _ = stock.Call(17)
	cur, _, _ := loadcursor.Call(0, 32512)
	class := u("AntigravityRUOfflineBeta")
	wc := wndClass{Size: uint32(unsafe.Sizeof(wndClass{})), Callback: syscall.NewCallback(wndProc), Instance: hinst, Cursor: cur, Background: 6, ClassName: class}
	if r, _, _ := register.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return
	}
	hwnd, _, _ = create.Call(0, uintptr(unsafe.Pointer(class)), ptr("Antigravity RU 0.6.1 Beta"), wsOverlapped, 0x80000000, 0x80000000, 760, 460, 0, 0, hinst, 0)
	if hwnd == 0 {
		return
	}
	control("STATIC", "ANTIGRAVITY RU  •  0.6.1 BETA", 0, 0, 22, 18, 700, 24, 0)
	control("STATIC", "Автономная локализация  |  Windows x64  |  Без сети и фоновых служб", 0, 0, 22, 45, 700, 22, 0)
	control("STATIC", "Папка с Antigravity.exe:", 0, 0, 22, 85, 700, 20, 0)
	edit = control("EDIT", locateDefault(), tabstop|esAutoHscroll, exClientEdge, 22, 108, 590, 28, 0)
	control("BUTTON", "Обзор…", tabstop, 0, 620, 108, 100, 28, idBrowse)
	control("BUTTON", "Проверить", tabstop, 0, 22, 154, 160, 36, idCheck)
	control("BUTTON", "Установить / обновить", tabstop, 0, 193, 154, 255, 36, idInstall)
	control("BUTTON", "Восстановить оригинал", tabstop, 0, 459, 154, 261, 36, idUndo)
	control("STATIC", "Журнал проверки и установки", 0, 0, 22, 209, 700, 20, 0)
	logBox = control("EDIT", "При запуске русификатор штатно закрывает Antigravity.\r\nНовые сборки принимаются только при совпадении проверенных загрузчиков.\r\nНе отключайте защиту приложения или Windows.", esMultiline|esAutoVscroll|esReadOnly|wsVscroll|tabstop, exClientEdge, 22, 234, 698, 160, 0)
	show.Call(hwnd, 5)
	update.Call(hwnd)
	postMessage.Call(hwnd, wmStartupClose, 0, 0)
	var m msg
	for {
		r, _, _ := getmsg.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		translate.Call(uintptr(unsafe.Pointer(&m)))
		dispatch.Call(uintptr(unsafe.Pointer(&m)))
	}
}
