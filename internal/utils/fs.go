package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

// EnsureDir creates path (and any parents) with perm if it does not already
// exist. Existing directories are left untouched.
func EnsureDir(path string, perm os.FileMode) error {
	if err := os.MkdirAll(path, perm); err != nil {
		return fmt.Errorf("create directory %s: %w", path, err)
	}
	return nil
}

// AtomicWrite writes data to path atomically. It first writes to a temporary
// file in the same directory, then renames it over the destination. This
// guarantees that no reader ever sees a partial write.
func AtomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, ".sshelf-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()

	if err := writeAndClose(tmp, data, perm); err != nil {
		os.Remove(tmpPath)
		return err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename %s to %s: %w", tmpPath, path, err)
	}

	return nil
}

// FileExists reports whether a regular file exists at path.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// DirExists reports whether a directory exists at path.
func DirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// writeAndClose writes data to f with the given permissions, then closes it.
// If Close returns an error after a successful write, that error is propagated.
func writeAndClose(f *os.File, data []byte, perm os.FileMode) (err error) {
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close %s: %w", f.Name(), cerr)
		}
	}()

	if err = f.Chmod(perm); err != nil {
		return fmt.Errorf("chmod %s: %w", f.Name(), err)
	}
	if _, err = f.Write(data); err != nil {
		return fmt.Errorf("write %s: %w", f.Name(), err)
	}
	return nil
}
