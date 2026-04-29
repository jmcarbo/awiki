// Package fsutil provides shared filesystem helpers (atomic write,
// advisory locking) used across awiki domain packages.
package fsutil

import (
	"os"
	"path/filepath"
)

// AtomicWrite replaces the file at path with data, preserving the
// existing file's mode bits. Implemented as write-to-temp + rename so
// readers never see a partial file. The target file must already exist;
// callers create new files with os.WriteFile.
func AtomicWrite(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	keepTemp := true
	defer func() {
		if keepTemp {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		return closeTempFile(tmp, err)
	}
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		return closeTempFile(tmp, err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	keepTemp = false
	return nil
}

func closeTempFile(file *os.File, err error) error {
	if closeErr := file.Close(); closeErr != nil && err == nil {
		return closeErr
	}
	return err
}
