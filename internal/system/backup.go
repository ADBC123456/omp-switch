package system

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func BackupFile(path, backupDir string) error {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("cannot back up directory %s", path)
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return err
	}
	filename := filepath.Base(path)
	dst, err := os.CreateTemp(backupDir, filename+"-"+time.Now().Format("2006-01-02-150405.000000000")+"-*.bak")
	if err != nil {
		return err
	}
	target := dst.Name()
	src, err := os.Open(path)
	if err != nil {
		_ = dst.Close()
		_ = os.Remove(target)
		return err
	}
	_, copyErr := io.Copy(dst, src)
	modeErr := dst.Chmod(0o644)
	closeDstErr := dst.Close()
	closeSrcErr := src.Close()
	if err := errors.Join(copyErr, modeErr, closeDstErr, closeSrcErr); err != nil {
		_ = os.Remove(target)
		return err
	}
	return trimBackups(backupDir, 20)
}

func trimBackups(dir string, keep int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) <= keep {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool {
		left, _ := entries[i].Info()
		right, _ := entries[j].Info()
		return left.ModTime().After(right.ModTime())
	})
	for _, entry := range entries[keep:] {
		_ = os.Remove(filepath.Join(dir, entry.Name()))
	}
	return nil
}
