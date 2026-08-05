package omp

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"
)

type LaunchPreview struct {
	Executable string   `json:"executable"`
	Arguments  []string `json:"arguments"`
}

func BuildLaunch(ompCommand, providerID, modelID string) (LaunchPreview, error) {
	executable, err := validateOMPCommand(ompCommand)
	if err != nil {
		return LaunchPreview{}, err
	}
	providerID = strings.TrimSpace(providerID)
	modelID = strings.TrimSpace(modelID)
	if providerID == "" || modelID == "" {
		return LaunchPreview{}, errors.New("Provider 和模型不能为空")
	}
	if strings.Contains(providerID, "/") || strings.IndexFunc(providerID+modelID, unicode.IsControl) >= 0 {
		return LaunchPreview{}, errors.New("Provider 或模型无效")
	}
	return LaunchPreview{Executable: executable, Arguments: []string{"--model", providerID + "/" + modelID}}, nil
}

func BuildResumeLaunch(ompCommand, sessionPath string) (LaunchPreview, error) {
	executable, err := validateOMPCommand(ompCommand)
	if err != nil {
		return LaunchPreview{}, err
	}
	sessionPath = strings.TrimSpace(sessionPath)
	if sessionPath == "" || strings.IndexFunc(sessionPath, unicode.IsControl) >= 0 {
		return LaunchPreview{}, errors.New("OMP 会话路径无效")
	}
	return LaunchPreview{Executable: executable, Arguments: []string{"--resume", sessionPath}}, nil
}

func validateOMPCommand(command string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", errors.New("OMP 命令不能为空")
	}
	if strings.IndexFunc(command, unicode.IsControl) >= 0 {
		return "", errors.New("OMP 命令不能包含控制字符")
	}
	return command, nil
}

func validateLaunchPreview(preview LaunchPreview) error {
	if _, err := validateOMPCommand(preview.Executable); err != nil {
		return err
	}
	if len(preview.Arguments) != 2 {
		return errors.New("OMP 启动参数无效")
	}
	if preview.Arguments[0] == "--model" {
		_, err := BuildLaunch(preview.Executable, previewProvider(preview), previewModel(preview))
		return err
	}
	if preview.Arguments[0] == "--resume" {
		_, err := BuildResumeLaunch(preview.Executable, preview.Arguments[1])
		return err
	}
	return errors.New("OMP 启动参数无效")
}

type ManagedProcess struct {
	cmd  *exec.Cmd
	done chan struct{}
	once sync.Once
}

func StartManaged(preview LaunchPreview, workingDir string) (*ManagedProcess, error) {
	if err := validateLaunchPreview(preview); err != nil {
		return nil, err
	}
	executable, err := resolveExecutable(preview.Executable)
	if err != nil {
		return nil, err
	}
	preview.Executable = executable
	cmd := managedCommand(preview, workingDir)
	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 OMP: %w", err)
	}
	process := &ManagedProcess{cmd: cmd, done: make(chan struct{})}
	go func() {
		_ = cmd.Wait()
		process.once.Do(func() { close(process.done) })
	}()
	select {
	case <-process.done:
		return nil, fmt.Errorf("OMP 启动后立即退出（命令：%s）", executable)
	case <-time.After(500 * time.Millisecond):
		return process, nil
	}
}

func resolveExecutable(command string) (string, error) {
	if executable, err := exec.LookPath(command); err == nil {
		return executable, nil
	}
	if runtime.GOOS == "windows" && strings.EqualFold(filepath.Base(command), "omp") {
		candidate := filepath.Join(os.Getenv("LOCALAPPDATA"), "omp", "omp.exe")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("找不到 OMP 命令 %q；请在设置中填写 omp.exe 的完整路径", command)
}

func (process *ManagedProcess) Running() bool {
	if process == nil {
		return false
	}
	select {
	case <-process.done:
		return false
	default:
		return true
	}
}

func (process *ManagedProcess) Stop() error {
	if process == nil || process.cmd == nil || process.cmd.Process == nil {
		return nil
	}
	select {
	case <-process.done:
		return nil
	default:
	}
	terminationErr := terminateProcessTree(process.cmd.Process.Pid)
	if terminationErr == nil {
		<-process.done
		return nil
	}
	select {
	case <-process.done:
		return nil
	case <-time.After(time.Second):
		return terminationErr
	}
}

func previewProvider(preview LaunchPreview) string {
	if len(preview.Arguments) != 2 || preview.Arguments[0] != "--model" {
		return ""
	}
	separator := strings.IndexByte(preview.Arguments[1], '/')
	if separator <= 0 {
		return ""
	}
	return preview.Arguments[1][:separator]
}

func previewModel(preview LaunchPreview) string {
	if len(preview.Arguments) != 2 || preview.Arguments[0] != "--model" {
		return ""
	}
	separator := strings.IndexByte(preview.Arguments[1], '/')
	if separator < 0 || separator == len(preview.Arguments[1])-1 {
		return ""
	}
	return preview.Arguments[1][separator+1:]
}

func powershellCommand(preview LaunchPreview, workingDir string) string {
	arguments := make([]string, len(preview.Arguments))
	for index, argument := range preview.Arguments {
		arguments[index] = windowsCommandLineArgument(argument)
	}
	command := "$process = Start-Process -FilePath " + powerShellQuote(preview.Executable)
	if len(arguments) > 0 {
		command += " -ArgumentList " + powerShellQuote(strings.Join(arguments, " "))
	}
	if workingDir != "" {
		command += " -WorkingDirectory " + powerShellQuote(workingDir)
	}
	return command + " -Wait -PassThru; exit $process.ExitCode"
}

func windowsCommandLineArgument(value string) string {
	if value == "" {
		return `""`
	}
	if !strings.ContainsAny(value, " \t\"") {
		return value
	}
	var quoted strings.Builder
	quoted.Grow(len(value) + 2)
	quoted.WriteByte('"')
	backslashes := 0
	for _, character := range value {
		switch character {
		case '\\':
			backslashes++
		case '"':
			quoted.WriteString(strings.Repeat(`\`, backslashes*2+1))
			quoted.WriteRune(character)
			backslashes = 0
		default:
			quoted.WriteString(strings.Repeat(`\`, backslashes))
			quoted.WriteRune(character)
			backslashes = 0
		}
	}
	quoted.WriteString(strings.Repeat(`\`, backslashes*2))
	quoted.WriteByte('"')
	return quoted.String()
}

func powerShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func appleScriptCommand(preview LaunchPreview) string {
	parts := make([]string, 0, 1+len(preview.Arguments))
	parts = append(parts, shellQuote(preview.Executable))
	for _, argument := range preview.Arguments {
		parts = append(parts, shellQuote(argument))
	}
	command := strings.Join(parts, " ")
	return `tell application "Terminal" to do script "` + appleScriptQuote(command) + `"`
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func appleScriptQuote(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}
