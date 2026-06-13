package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteFileUsesTargetDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := AtomicWriteFile(path, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWriteFile(path, []byte("second"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "second" {
		t.Fatalf("content = %q", got)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".config.toml.tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files left behind: %#v", matches)
	}
}

func TestFileErrorDetailsUseOSErrors(t *testing.T) {
	err := &os.PathError{Op: "open", Path: "C:/tmp/config.toml", Err: errors.New("access denied")}
	details := FileErrorDetails(err)
	if details["op"] != "open" || details["path"] != "C:/tmp/config.toml" || details["os_error"] != "access denied" {
		t.Fatalf("details = %#v", details)
	}
	if _, ok := details["errno"]; ok {
		t.Fatalf("details should use os_error_code instead of errno: %#v", details)
	}
}
