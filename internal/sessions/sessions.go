package sessions

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Info struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	WorkingDir string `json:"workingDir"`
	Model      string `json:"model"`
	UpdatedAt  string `json:"updatedAt"`
	SizeBytes  int64  `json:"sizeBytes"`
}

type Manager struct {
	root string
}

type record struct {
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updatedAt"`
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	CWD       string    `json:"cwd"`
	Model     string    `json:"model"`
	Message   struct {
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
}

type storedSession struct {
	Info
	path      string
	updatedAt time.Time
}

func NewManager(root string) *Manager {
	return &Manager{root: filepath.Clean(root)}
}

func (manager *Manager) List() ([]Info, error) {
	sessions, err := manager.listStored()
	if err != nil {
		return nil, err
	}
	result := make([]Info, len(sessions))
	for index := range sessions {
		result[index] = sessions[index].Info
	}
	return result, nil
}

func (manager *Manager) Find(id string) (Info, string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Info{}, "", errors.New("会话 ID 不能为空")
	}
	sessions, err := manager.listStored()
	if err != nil {
		return Info{}, "", err
	}
	for _, item := range sessions {
		if item.ID == id {
			return item.Info, item.path, nil
		}
	}
	return Info{}, "", errors.New("未找到 OMP 会话")
}

func (manager *Manager) Delete(id string) error {
	_, path, err := manager.Find(id)
	if err != nil {
		return err
	}
	attachments := strings.TrimSuffix(path, filepath.Ext(path))
	if err = os.RemoveAll(attachments); err != nil {
		return fmt.Errorf("删除会话附件: %w", err)
	}
	if err = os.Remove(path); err != nil {
		return fmt.Errorf("删除会话文件: %w", err)
	}
	return nil
}

func (manager *Manager) listStored() ([]storedSession, error) {
	if manager == nil || manager.root == "." || strings.TrimSpace(manager.root) == "" {
		return nil, errors.New("OMP 会话目录不能为空")
	}
	if _, err := os.Stat(manager.root); errors.Is(err, os.ErrNotExist) {
		return []storedSession{}, nil
	} else if err != nil {
		return nil, fmt.Errorf("检查 OMP 会话目录: %w", err)
	}

	result := make([]storedSession, 0)
	projects, err := os.ReadDir(manager.root)
	if err != nil {
		return nil, fmt.Errorf("读取 OMP 会话目录: %w", err)
	}
	for _, project := range projects {
		if !project.IsDir() || project.Type()&os.ModeSymlink != 0 {
			continue
		}
		projectPath := filepath.Join(manager.root, project.Name())
		entries, readErr := os.ReadDir(projectPath)
		if readErr != nil {
			return nil, fmt.Errorf("读取 OMP 项目会话目录: %w", readErr)
		}
		for _, entry := range entries {
			if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.EqualFold(filepath.Ext(entry.Name()), ".jsonl") {
				continue
			}
			item, parseErr := readSession(filepath.Join(projectPath, entry.Name()))
			if parseErr == nil {
				result = append(result, item)
			}
		}
	}
	sort.Slice(result, func(left, right int) bool {
		return result[left].updatedAt.After(result[right].updatedAt)
	})
	return result, nil
}

func readSession(path string) (storedSession, error) {
	file, err := os.Open(path)
	if err != nil {
		return storedSession{}, err
	}
	defer file.Close()

	var item storedSession
	var firstUserMessage string
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	lines := 0
	for scanner.Scan() {
		lines++
		var entry record
		if err = json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			if lines >= 100 {
				break
			}
			continue
		}
		switch entry.Type {
		case "title":
			if title := strings.TrimSpace(entry.Title); title != "" {
				item.Title = title
			}
			if entry.UpdatedAt.After(item.updatedAt) {
				item.updatedAt = entry.UpdatedAt
			}
		case "session":
			item.ID = strings.TrimSpace(entry.ID)
			item.WorkingDir = strings.TrimSpace(entry.CWD)
			if entry.Timestamp.After(item.updatedAt) {
				item.updatedAt = entry.Timestamp
			}
			if item.Title == "" {
				item.Title = strings.TrimSpace(entry.Title)
			}
		case "model_change":
			if entry.Model != "" {
				item.Model = entry.Model
			}
		case "message":
			if firstUserMessage == "" && entry.Message.Role == "user" {
				for _, content := range entry.Message.Content {
					if content.Type == "text" && strings.TrimSpace(content.Text) != "" {
						firstUserMessage = strings.TrimSpace(content.Text)
						break
					}
				}
			}
		}
		if item.ID != "" && item.WorkingDir != "" && item.Model != "" && (item.Title != "" || firstUserMessage != "") {
			break
		}
		if lines >= 100 {
			break
		}
	}
	if err = scanner.Err(); err != nil && (item.ID == "" || item.WorkingDir == "") {
		return storedSession{}, err
	}
	if item.ID == "" || item.WorkingDir == "" {
		return storedSession{}, errors.New("无效的 OMP 会话文件")
	}
	stat, err := file.Stat()
	if err != nil {
		return storedSession{}, err
	}
	item.path = path
	item.SizeBytes = stat.Size()
	if stat.ModTime().After(item.updatedAt) {
		item.updatedAt = stat.ModTime()
	}
	item.UpdatedAt = item.updatedAt.Format(time.RFC3339Nano)
	if item.Title == "" {
		item.Title = compactTitle(firstUserMessage)
	}
	if item.Title == "" {
		item.Title = "未命名会话"
	}
	return item, nil
}

func compactTitle(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	const maxRunes = 80
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "…"
}
