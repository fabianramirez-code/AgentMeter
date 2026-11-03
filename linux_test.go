//go:build linux

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProcessRunningIn(t *testing.T) {
	proc := t.TempDir()
	for pid, comm := range map[string]string{
		"1":    "systemd\n",
		"4242": "claude\n",
		"77":   "a-very-long-pro\n", // the kernel truncates comm to 15 bytes
	} {
		if err := os.MkdirAll(filepath.Join(proc, pid), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(proc, pid, "comm"), []byte(comm), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// not a pid: must be ignored
	if err := os.MkdirAll(filepath.Join(proc, "self"), 0o755); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		want bool
	}{
		{"claude", true},
		{"Claude", true},
		{"codex", false},
		{"a-very-long-process-name", true},
	} {
		if got := processRunningIn(proc, tc.name); got != tc.want {
			t.Errorf("processRunningIn(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}
