package main

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

// A Windows named mutex also covers separate optimizer instances and is
// released by the OS after a crash, so a stale lock cannot disable updates.
func lockScannerInstall(root string) (func(), error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(abs))))
	name, err := syscall.UTF16PtrFromString(fmt.Sprintf("Local\\ZZZScannerUpdate-%x", hash[:16]))
	if err != nil {
		return nil, err
	}
	kernel := syscall.NewLazyDLL("kernel32.dll")
	handle, _, callErr := kernel.NewProc("CreateMutexW").Call(0, 0, uintptr(unsafe.Pointer(name)))
	if handle == 0 {
		return nil, fmt.Errorf("无法锁定 Scanner 安装：%w", callErr)
	}
	if callErr == syscall.ERROR_ALREADY_EXISTS {
		syscall.CloseHandle(syscall.Handle(handle))
		return nil, fmt.Errorf("另一个配装器正在检查或更新 Scanner")
	}
	return func() {
		syscall.CloseHandle(syscall.Handle(handle))
	}, nil
}

// Include scanners launched outside this optimizer, such as a second instance
// or an elevated scanner that cannot be tracked through scannerRuntime.cmd.
func scannerProcessRunning() (bool, error) {
	snapshot, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false, err
	}
	defer syscall.CloseHandle(snapshot)
	entry := syscall.ProcessEntry32{}
	entry.Size = uint32(unsafe.Sizeof(entry))
	err = syscall.Process32First(snapshot, &entry)
	for err == nil {
		if strings.EqualFold(syscall.UTF16ToString(entry.ExeFile[:]), "ZZZ-Scanner.Next.exe") {
			return true, nil
		}
		err = syscall.Process32Next(snapshot, &entry)
	}
	if err == syscall.ERROR_NO_MORE_FILES {
		return false, nil
	}
	return false, err
}
