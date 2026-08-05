package skills

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const maxManifestSize = 64 << 10

type Info struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
	Locked      bool   `json:"locked"`
}

type Inventory struct {
	Root   string `json:"root"`
	Skills []Info `json:"skills"`
}

type Manager struct {
	Root     string
	LockPath string
}

func (manager Manager) List() (Inventory, error) {
	root, err := filepath.Abs(manager.Root)
	if err != nil {
		return Inventory{}, err
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return Inventory{Root: root, Skills: []Info{}}, nil
	}
	if err != nil {
		return Inventory{}, fmt.Errorf("读取全局 Skill 目录: %w", err)
	}
	locked, err := manager.lockedSkills()
	if err != nil {
		return Inventory{}, err
	}
	items := make([]Info, 0, len(entries))
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		stat, statErr := os.Stat(path)
		if statErr != nil || !stat.IsDir() {
			continue
		}
		manifest := filepath.Join(path, "SKILL.md")
		if manifestStat, manifestErr := os.Stat(manifest); manifestErr != nil || manifestStat.IsDir() {
			continue
		}
		items = append(items, Info{
			Name: entry.Name(), Description: manifestDescription(manifest), Path: path,
			Locked: locked[entry.Name()],
		})
	}
	sort.Slice(items, func(i, j int) bool { return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name) })
	return Inventory{Root: root, Skills: items}, nil
}

func (manager Manager) Delete(name string) (Inventory, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) {
		return Inventory{}, errors.New("Skill 名称无效")
	}
	root, err := filepath.Abs(manager.Root)
	if err != nil {
		return Inventory{}, err
	}
	target := filepath.Join(root, name)
	if filepath.Dir(target) != root {
		return Inventory{}, errors.New("Skill 路径不在全局目录内")
	}
	stat, err := os.Stat(target)
	if errors.Is(err, os.ErrNotExist) {
		return Inventory{}, errors.New("全局 Skill 不存在：" + name)
	}
	if err != nil {
		return Inventory{}, fmt.Errorf("检查全局 Skill: %w", err)
	}
	if !stat.IsDir() {
		return Inventory{}, errors.New("全局 Skill 不是目录：" + name)
	}
	if manifest, manifestErr := os.Stat(filepath.Join(target, "SKILL.md")); manifestErr != nil || manifest.IsDir() {
		return Inventory{}, errors.New("目录不是有效的全局 Skill：" + name)
	}

	tombstone := filepath.Join(root, fmt.Sprintf(".ompswitch-delete-%d", time.Now().UnixNano()))
	if err = os.Rename(target, tombstone); err != nil {
		return Inventory{}, fmt.Errorf("移出全局 Skill: %w", err)
	}
	if err = manager.removeLockEntry(name); err != nil {
		if restoreErr := os.Rename(tombstone, target); restoreErr != nil {
			return Inventory{}, errors.Join(err, fmt.Errorf("恢复 Skill 目录: %w", restoreErr))
		}
		return Inventory{}, err
	}
	if err = os.RemoveAll(tombstone); err != nil {
		return Inventory{}, fmt.Errorf("删除全局 Skill: %w", err)
	}
	return manager.List()
}

func (manager Manager) lockedSkills() (map[string]bool, error) {
	locked := map[string]bool{}
	if strings.TrimSpace(manager.LockPath) == "" {
		return locked, nil
	}
	data, err := os.ReadFile(manager.LockPath)
	if errors.Is(err, os.ErrNotExist) {
		return locked, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取全局 Skill 锁文件: %w", err)
	}
	var document map[string]json.RawMessage
	if err = json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("解析全局 Skill 锁文件: %w", err)
	}
	var records map[string]json.RawMessage
	if raw := document["skills"]; len(raw) > 0 {
		if err = json.Unmarshal(raw, &records); err != nil {
			return nil, fmt.Errorf("解析全局 Skill 锁记录: %w", err)
		}
	}
	for name := range records {
		locked[name] = true
	}
	return locked, nil
}

func (manager Manager) removeLockEntry(name string) error {
	if strings.TrimSpace(manager.LockPath) == "" {
		return nil
	}
	data, err := os.ReadFile(manager.LockPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取全局 Skill 锁文件: %w", err)
	}
	var document map[string]json.RawMessage
	if err = json.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("解析全局 Skill 锁文件: %w", err)
	}
	var records map[string]json.RawMessage
	if raw := document["skills"]; len(raw) > 0 {
		if err = json.Unmarshal(raw, &records); err != nil {
			return fmt.Errorf("解析全局 Skill 锁记录: %w", err)
		}
	}
	if _, exists := records[name]; !exists {
		return nil
	}
	delete(records, name)
	document["skills"], err = json.Marshal(records)
	if err != nil {
		return err
	}
	updated, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	updated = append(updated, '\n')
	mode := os.FileMode(0o600)
	if info, statErr := os.Stat(manager.LockPath); statErr == nil {
		mode = info.Mode().Perm()
	}
	temporary, err := os.CreateTemp(filepath.Dir(manager.LockPath), ".skill-lock-*.tmp")
	if err != nil {
		return fmt.Errorf("创建 Skill 锁临时文件: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err = temporary.Chmod(mode); err == nil {
		_, err = temporary.Write(updated)
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("写入 Skill 锁临时文件: %w", err)
	}
	backupPath := manager.LockPath + fmt.Sprintf(".ompswitch-backup-%d", time.Now().UnixNano())
	if err = os.Rename(manager.LockPath, backupPath); err != nil {
		return fmt.Errorf("备份全局 Skill 锁文件: %w", err)
	}
	if err = os.Rename(temporaryPath, manager.LockPath); err != nil {
		restoreErr := os.Rename(backupPath, manager.LockPath)
		return errors.Join(fmt.Errorf("更新全局 Skill 锁文件: %w", err), restoreErr)
	}
	_ = os.Remove(backupPath)
	return nil
}

func manifestDescription(path string) string {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return ""
	}
	if len(data) > maxManifestSize {
		data = data[:maxManifestSize]
	}
	text := string(data)
	if !strings.HasPrefix(text, "---\n") && !strings.HasPrefix(text, "---\r\n") {
		return ""
	}
	text = strings.TrimPrefix(strings.TrimPrefix(text, "---\r\n"), "---\n")
	end := strings.Index(text, "\n---")
	if end < 0 {
		return ""
	}
	var frontmatter struct {
		Description string `yaml:"description"`
	}
	if yaml.Unmarshal([]byte(text[:end]), &frontmatter) != nil {
		return ""
	}
	return strings.TrimSpace(frontmatter.Description)
}
