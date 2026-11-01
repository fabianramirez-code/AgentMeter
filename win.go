//go:build windows

package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// isProcessRunning reports whether a process with the given executable name
// (e.g. "claude.exe") is currently running. Thin OS I/O, untested.
func isProcessRunning(name string) bool {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	if err := windows.Process32First(snapshot, &entry); err != nil {
		return false
	}
	target := strings.ToLower(name)
	for {
		exe := strings.ToLower(windows.UTF16ToString(entry.ExeFile[:]))
		if exe == target {
			return true
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			return false
		}
	}
}

// newestModTime walks dir recursively and returns the newest file ModTime,
// or the zero time if dir does not exist or is empty. Thin OS I/O, untested.
func newestModTime(dir string) time.Time {
	var newest time.Time
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
		return nil
	})
	return newest
}

// advapi32 CredReadW/CredFree, called directly since x/sys/windows has no
// Credential Manager wrapper.
var (
	advapi32     = windows.NewLazySystemDLL("advapi32.dll")
	procCredRead = advapi32.NewProc("CredReadW")
	procCredFree = advapi32.NewProc("CredFree")
)

const credTypeGeneric = 1

// winCredential mirrors the Win32 CREDENTIAL struct layout we need.
type winCredential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

// readCredentialBlob reads a generic Windows Credential Manager credential
// and returns its raw blob bytes. Thin OS I/O, untested.
func readCredentialBlob(target string) ([]byte, error) {
	targetPtr, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return nil, err
	}
	var pCred *winCredential
	ret, _, _ := procCredRead.Call(
		uintptr(unsafe.Pointer(targetPtr)),
		uintptr(credTypeGeneric),
		0,
		uintptr(unsafe.Pointer(&pCred)),
	)
	if ret == 0 {
		return nil, fmt.Errorf("CredReadW failed for target %q", target)
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(pCred)))

	blob := unsafe.Slice(pCred.CredentialBlob, int(pCred.CredentialBlobSize))
	out := make([]byte, len(blob))
	copy(out, blob)
	return out, nil
}
