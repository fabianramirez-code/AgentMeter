package main

import (
	"io/fs"
	"path/filepath"
	"time"
)

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
