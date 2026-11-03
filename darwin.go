//go:build darwin

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/sys/unix"
)

// defaultProcs is the -procs default: the CLI executable names on macOS.
const defaultProcs = "claude=claude;codex=codex;gemini=agy"

// NOTE: on macOS Claude Code keeps its OAuth JSON (same shape as the
// Windows .credentials.json) in the login keychain, generic password with
// service "Claude Code-credentials". Not yet verified on a real Mac.
const claudeKeychainService = "Claude Code-credentials"

// readClaudeCredentials returns the Claude Code OAuth JSON: from the
// keychain, or from credPath when the CLI fell back to a plain file.
// Thin OS I/O, untested.
func readClaudeCredentials(credPath string) ([]byte, error) {
	if blob, err := readCredentialBlob(claudeKeychainService); err == nil {
		return blob, nil
	}
	return os.ReadFile(credPath)
}

// isProcessRunning reports whether a process with the given executable name
// (e.g. "claude") is currently running. The kernel keeps only the first 16
// bytes of the name (MAXCOMLEN), so the target is truncated the same way.
// Thin OS I/O, untested.
func isProcessRunning(name string) bool {
	procs, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return false
	}
	target := strings.ToLower(name)
	if len(target) > 16 {
		target = target[:16]
	}
	for _, p := range procs {
		comm := p.Proc.P_comm[:]
		if i := bytes.IndexByte(comm, 0); i >= 0 {
			comm = comm[:i]
		}
		if strings.ToLower(string(comm)) == target {
			return true
		}
	}
	return false
}

// readCredentialBlob reads a generic password from the login keychain.
// target uses the Windows Credential Manager naming, "service:account" as
// written by go-keyring, or a bare service name. Thin OS I/O, untested.
func readCredentialBlob(target string) ([]byte, error) {
	args := []string{"find-generic-password", "-w"}
	if service, account, ok := strings.Cut(target, ":"); ok {
		args = append(args, "-s", service, "-a", account)
	} else {
		args = append(args, "-s", target)
	}
	out, err := exec.Command("/usr/bin/security", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("keychain item %q not found: %w", target, err)
	}
	return decodeKeyringSecret(strings.TrimRight(string(out), "\n"))
}
