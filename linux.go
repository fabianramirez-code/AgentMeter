//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// defaultProcs is the -procs default: the CLI executable names on Linux.
const defaultProcs = "claude=claude;codex=codex;gemini=agy"

// readClaudeCredentials returns the Claude Code OAuth JSON. On Linux the
// CLI keeps it in a plain file, as on Windows. Thin OS I/O, untested.
func readClaudeCredentials(credPath string) ([]byte, error) {
	return os.ReadFile(credPath)
}

// isProcessRunning reports whether a process with the given executable name
// (e.g. "claude") is currently running.
func isProcessRunning(name string) bool {
	return processRunningIn("/proc", name)
}

// processRunningIn scans procDir/<pid>/comm. The kernel keeps only the
// first 15 bytes of the name (TASK_COMM_LEN - 1), so the target is
// truncated the same way.
func processRunningIn(procDir, name string) bool {
	target := strings.ToLower(name)
	if len(target) > 15 {
		target = target[:15]
	}
	comms, _ := filepath.Glob(filepath.Join(procDir, "[0-9]*", "comm"))
	for _, path := range comms {
		comm, err := os.ReadFile(path)
		if err != nil {
			continue // process exited while scanning
		}
		if strings.ToLower(strings.TrimSpace(string(comm))) == target {
			return true
		}
	}
	return false
}

// readCredentialBlob reads a secret from the Secret Service (GNOME Keyring,
// KWallet) with secret-tool (package libsecret-tools / libsecret). target
// uses the Windows Credential Manager naming, "service:account" as written
// by go-keyring, which stores the attributes "service" and "username".
// Thin OS I/O, untested.
func readCredentialBlob(target string) ([]byte, error) {
	service, account, _ := strings.Cut(target, ":")
	args := []string{"lookup", "service", service}
	if account != "" {
		args = append(args, "username", account)
	}
	out, err := exec.Command("secret-tool", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("secret %q not found (is secret-tool installed?): %w", target, err)
	}
	return decodeKeyringSecret(strings.TrimRight(string(out), "\n"))
}
