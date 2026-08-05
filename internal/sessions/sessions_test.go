package sessions

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeSession(t *testing.T, root, directory, name, id, title, cwd, model string, modified time.Time) string {
	t.Helper()
	path := filepath.Join(root, directory, name+".jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := `{"type":"title","title":"` + title + `","updatedAt":"` + modified.Format(time.RFC3339Nano) + `"}` + "\n" +
		`{"type":"session","id":"` + id + `","timestamp":"` + modified.Add(-time.Minute).Format(time.RFC3339Nano) + `","cwd":"` + filepath.ToSlash(cwd) + `"}` + "\n" +
		`{"type":"model_change","model":"` + model + `"}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modified, modified); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestListReturnsNewestSessionsWithMetadata(t *testing.T) {
	root := t.TempDir()
	older := time.Now().Add(-2 * time.Hour).UTC()
	newer := time.Now().Add(-time.Hour).UTC()
	writeSession(t, root, "project-a", "older", "session-old", "Old title", `C:\work\old`, "provider/old", older)
	writeSession(t, root, "project-b", "newer", "session-new", "New title", `C:\work\new`, "provider/new", newer)
	if err := os.WriteFile(filepath.Join(root, "broken.jsonl"), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeSession(t, root, filepath.Join("project-b", "newer"), "child", "child-session", "Internal child", `C:\work\child`, "provider/child", time.Now().UTC())

	items, err := NewManager(root).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != "session-new" || items[0].Title != "New title" || items[0].Model != "provider/new" {
		t.Fatalf("sessions = %#v", items)
	}
}

func TestListFallsBackToFirstUserMessageTitle(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "project", "session.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "{\"type\":\"title\",\"title\":\"\"}\n" +
		"{\"type\":\"session\",\"id\":\"session-id\",\"timestamp\":\"2026-08-05T04:00:19Z\",\"cwd\":\"C:\\\\work\"}\n" +
		"{\"type\":\"model_change\",\"model\":\"provider/model\"}\n" +
		"{\"type\":\"message\",\"message\":{\"role\":\"user\",\"content\":[{\"type\":\"text\",\"text\":\"  Fix   the session manager  \"}]}}\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	items, err := NewManager(root).List()
	if err != nil || len(items) != 1 || items[0].Title != "Fix the session manager" {
		t.Fatalf("sessions = %#v, err = %v", items, err)
	}
}

func TestDeleteRemovesSessionAndAttachmentDirectory(t *testing.T) {
	root := t.TempDir()
	path := writeSession(t, root, "project", "session", "session-id", "Title", t.TempDir(), "provider/model", time.Now().UTC())
	attachments := path[:len(path)-len(filepath.Ext(path))]
	if err := os.MkdirAll(attachments, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachments, "artifact.txt"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(root)
	if err := manager.Delete("session-id"); err != nil {
		t.Fatal(err)
	}
	for _, removed := range []string{path, attachments} {
		if _, err := os.Stat(removed); !os.IsNotExist(err) {
			t.Fatalf("%s still exists: %v", removed, err)
		}
	}
	outside := filepath.Join(t.TempDir(), "keep.txt")
	if err := os.WriteFile(outside, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := manager.Delete(outside); err == nil {
		t.Fatal("unknown path was accepted as a session ID")
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside file changed: %v", err)
	}
}
