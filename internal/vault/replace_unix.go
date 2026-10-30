//go:build !windows

package vault

import (
	"os"
	"path/filepath"
)

func replaceFile(from, to string) error {
	if e := os.Rename(from, to); e != nil {
		return e
	}
	if dir, e := os.Open(filepath.Dir(to)); e == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}
