//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	snapshot        = k32.NewProc("CreateToolhelp32Snapshot")
	firstProcess    = k32.NewProc("Process32FirstW")
	nextProcess     = k32.NewProc("Process32NextW")
	closeHandle     = k32.NewProc("CloseHandle")
	openProcess     = k32.NewProc("OpenProcess")
	queryImage      = k32.NewProc("QueryFullProcessImageNameW")
	enumWindows     = u32.NewProc("EnumWindows")
	windowPID       = u32.NewProc("GetWindowThreadProcessId")
	isWindowVisible = u32.NewProc("IsWindowVisible")
	postMessage     = u32.NewProc("PostMessageW")
)

type processEntry struct {
	Size, Usage, PID             uint32
	DefaultHeap                  uintptr
	ModuleID, Threads, ParentPID uint32
	Priority                     int32
	Flags                        uint32
	Name                         [260]uint16
}

func antigravityPIDs() ([]uint32, error) {
	h, _, err := snapshot.Call(2, 0) // TH32CS_SNAPPROCESS
	if h == ^uintptr(0) {
		return nil, fmt.Errorf("не удалось получить список процессов: %w", err)
	}
	defer closeHandle.Call(h)
	var p processEntry
	p.Size = uint32(unsafe.Sizeof(p))
	var ids []uint32
	r, _, err := firstProcess.Call(h, uintptr(unsafe.Pointer(&p)))
	for r != 0 {
		if strings.EqualFold(syscall.UTF16ToString(p.Name[:]), "Antigravity.exe") {
			ids = append(ids, p.PID)
		}
		r, _, err = nextProcess.Call(h, uintptr(unsafe.Pointer(&p)))
	}
	if err != syscall.ERROR_NO_MORE_FILES {
		return nil, fmt.Errorf("не удалось проверить список процессов: %w", err)
	}
	return ids, nil
}

func isStopped() (bool, error) {
	ids, err := antigravityPIDs()
	return len(ids) == 0 && err == nil, err
}
func ensureStopped() error {
	done, err := isStopped()
	if err != nil {
		return err
	}
	if !done {
		return fmt.Errorf("Antigravity запущен. Изменение файлов заблокировано.")
	}
	return nil
}

// Request WM_CLOSE only for visible windows belonging to Antigravity.exe
// in the selected installation. Do not touch other copies or processes.
func requestNormalClose(root string) error {
	if !filepath.IsAbs(root) {
		return fmt.Errorf("не указан абсолютный путь установки")
	}
	target := filepath.Join(root, "Antigravity.exe")
	st, err := os.Lstat(target)
	if err != nil || !st.Mode().IsRegular() {
		return fmt.Errorf("Antigravity.exe не найден в выбранной папке")
	}
	target, err = filepath.EvalSymlinks(target)
	if err != nil {
		return err
	}
	ids, err := antigravityPIDs()
	if err != nil {
		return err
	}
	eligible := map[uint32]uintptr{}
	defer func() {
		for _, h := range eligible {
			closeHandle.Call(h)
		}
	}()
	for _, pid := range ids {
		h, _, _ := openProcess.Call(0x1000, 0, uintptr(pid)) // PROCESS_QUERY_LIMITED_INFORMATION
		if h == 0 {
			continue
		} // cannot verify identity, never send close
		var buf [32768]uint16
		n := uint32(len(buf))
		ok, _, _ := queryImage.Call(h, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))
		if ok == 0 || n == 0 || n >= uint32(len(buf)) {
			closeHandle.Call(h)
			continue
		}
		path := syscall.UTF16ToString(buf[:n])
		canonical, e := filepath.EvalSymlinks(path)
		if e != nil || !strings.EqualFold(filepath.Clean(canonical), filepath.Clean(target)) {
			closeHandle.Call(h)
			continue
		}
		eligible[pid] = h
	}
	// One reusable callback, no per-call callback allocations.
	var postErr error
	activeCloseTargets = eligible
	activeCloseError = &postErr
	defer func() { activeCloseTargets = nil; activeCloseError = nil }()
	r, _, err := enumWindows.Call(closeWindowCallback, 0)
	if r == 0 {
		return fmt.Errorf("не удалось перечислить окна: %w", err)
	}
	return postErr
}

// Used exclusively on the GUI thread; no background callers.
var activeCloseTargets map[uint32]uintptr
var activeCloseError *error
var closeWindowCallback = syscall.NewCallback(func(w, _ uintptr) uintptr {
	var pid uint32
	windowPID.Call(w, uintptr(unsafe.Pointer(&pid)))
	if _, ok := activeCloseTargets[pid]; !ok {
		return 1
	}
	if visible, _, _ := isWindowVisible.Call(w); visible == 0 {
		return 1
	}
	// WM_CLOSE lets the app handle unsaved work and confirmation itself.
	if ok, _, err := postMessage.Call(w, wmClose, 0, 0); ok == 0 && activeCloseError != nil {
		*activeCloseError = fmt.Errorf("не удалось запросить штатное закрытие: %w", err)
	}
	return 1
})

func stopForPatch(root string) error {
	return closeAndWait(isStopped, func() error { return requestNormalClose(root) }, 8*time.Second, time.Sleep)
}
