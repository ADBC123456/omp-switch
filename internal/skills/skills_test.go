package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestListAndDeleteGlobalSkillUpdatesLock(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".agents", "skills")
	lockPath := filepath.Join(filepath.Dir(root), ".skill-lock.json")
	if err := os.MkdirAll(filepath.Join(root, "alpha"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "---\nname: alpha\ndescription: Short skill.\n---\nBody\n"
	if err := os.WriteFile(filepath.Join(root, "alpha", "SKILL.md"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	lock := `{"version":3,"skills":{"alpha":{"source":"test"}},"dismissed":{"keep":true}}`
	if err := os.WriteFile(lockPath, []byte(lock), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := Manager{Root: root, LockPath: lockPath}
	inventory, err := manager.List()
	if err != nil || len(inventory.Skills) != 1 || inventory.Skills[0].Description != "Short skill." || !inventory.Skills[0].Locked {
		t.Fatalf("inventory=%#v err=%v", inventory, err)
	}
	inventory, err = manager.Delete("alpha")
	if err != nil || len(inventory.Skills) != 0 {
		t.Fatalf("inventory after delete=%#v err=%v", inventory, err)
	}
	if _, err = os.Stat(filepath.Join(root, "alpha")); !os.IsNotExist(err) {
		t.Fatalf("skill directory still exists: %v", err)
	}
	data, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Skills    map[string]json.RawMessage `json:"skills"`
		Dismissed map[string]bool            `json:"dismissed"`
	}
	if err = json.Unmarshal(data, &document); err != nil || len(document.Skills) != 0 || !document.Dismissed["keep"] {
		t.Fatalf("lock=%s err=%v", data, err)
	}
}

func TestDeleteRejectsPathsAndNonSkills(t *testing.T) {
	root := t.TempDir()
	manager := Manager{Root: root}
	if _, err := manager.Delete("../outside"); err == nil {
		t.Fatal("path traversal accepted")
	}
	if err := os.Mkdir(filepath.Join(root, "plain"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Delete("plain"); err == nil {
		t.Fatal("directory without SKILL.md accepted")
	}
}
