package system_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ompswitch/internal/system"
)

func TestBackupFileUsesProvidedDirectory(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "state", "config.json")
	backupDir := filepath.Join(root, "chosen-backups")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := system.BackupFile(source, backupDir); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), "config.json-") {
		t.Fatalf("backups=%v", entries)
	}
	data, err := os.ReadFile(filepath.Join(backupDir, entries[0].Name()))
	if err != nil || string(data) != "before" {
		t.Fatalf("backup=%q, %v", data, err)
	}
}

func TestBackupFileMissingSourceIsNoOp(t *testing.T) {
	root := t.TempDir()
	backupDir := filepath.Join(root, "backups")
	if err := system.BackupFile(filepath.Join(root, "missing"), backupDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(backupDir); !os.IsNotExist(err) {
		t.Fatalf("backup directory created for missing source: %v", err)
	}
}

func TestBackupFileKeepsAtMostTwenty(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "config.json")
	backupDir := filepath.Join(root, "backups")
	if err := os.WriteFile(source, []byte("state"), 0o644); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 21; index++ {
		if err := system.BackupFile(source, backupDir); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 20 {
		t.Fatalf("backup count=%d, want 20", len(entries))
	}
}
