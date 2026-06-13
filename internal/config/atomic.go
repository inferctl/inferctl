package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

func AtomicWriteFile(path string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func FileErrorDetails(err error) map[string]any {
	details := map[string]any{
		"os_error": err.Error(),
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		details["op"] = pathErr.Op
		details["path"] = pathErr.Path
		details["os_error"] = pathErr.Err.Error()
		if errno, ok := pathErr.Err.(syscall.Errno); ok {
			details["os_error_code"] = strconv.FormatInt(int64(errno), 10)
		}
	}
	return details
}
