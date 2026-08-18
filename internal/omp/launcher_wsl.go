package omp

import (
	"errors"
	"strings"
)

// WindowsToWSLPath maps a Windows drive path to its WSL mount point form:
//
//	E:\\woker\\project  →  /mnt/e/woker/project
//
// UNC and relative paths are returned with backslashes converted as a best
// effort (WSL only auto-mounts drive letters under /mnt).
func WindowsToWSLPath(winPath string) string {
	winPath = strings.TrimSpace(winPath)
	if len(winPath) >= 2 && winPath[1] == ':' {
		drive := strings.ToLower(string(winPath[0]))
		rest := strings.Trim(filepathToSlash(winPath[2:]), "/")
		if rest == "" {
			return "/mnt/" + drive
		}
		return "/mnt/" + drive + "/" + rest
	}
	return filepathToSlash(winPath)
}

// WSLToWindowsPath maps a WSL /mnt path back to a Windows drive path:
//
//	/mnt/e/woker  →  E:\\woker
//
// Returns "" for WSL-internal paths (e.g. /home/user) that have no Windows
// counterpart.
func WSLToWindowsPath(wslPath string) string {
	wslPath = strings.TrimSpace(wslPath)
	lower := strings.ToLower(wslPath)
	if strings.HasPrefix(lower, "/mnt/") && len(wslPath) >= 6 {
		drive := strings.ToUpper(wslPath[5:6])
		rest := filepathFromSlash(wslPath[6:])
		return drive + ":" + rest
	}
	return ""
}

// WSLUNCToWSLPath maps a UNC path pointing inside a WSL distro back to the
// Linux path omp sees inside that distro:
//
//	\\wsl.localhost\\Ubuntu\\home\\admin\\.omp\\agent\\sessions\\p\\s.jsonl
//	    →  /home/admin/.omp/agent/sessions/p/s.jsonl
//	\\wsl$\\Ubuntu\\home\\admin\\.omp\\...  →  /home/admin/.omp/...
//
// Returns "" when the path is not a WSL UNC path for the given distro (or the
// distro is unknown) — callers should treat that as "cannot resume from the
// Windows side".
func WSLUNCToWSLPath(uncPath, distro string) string {
	uncPath = strings.TrimSpace(uncPath)
	distro = strings.TrimSpace(distro)
	if uncPath == "" || distro == "" {
		return ""
	}
	lowerPath := strings.ToLower(uncPath)
	lowerDistro := strings.ToLower(distro)
	for _, prefix := range []string{`\\wsl.localhost\`, `\\wsl$\`} {
		prefixLen := len(prefix)
		if !strings.HasPrefix(lowerPath, prefix) {
			continue
		}
		rest := lowerPath[prefixLen:]
		if !strings.HasPrefix(rest, lowerDistro+`\`) {
			continue
		}
		inside := uncPath[prefixLen+len(lowerDistro)+1:]
		inside = strings.ReplaceAll(inside, "\\", "/")
		return "/" + strings.Trim(inside, "/")
	}
	return ""
}

func filepathToSlash(p string) string   { return strings.ReplaceAll(p, "\\", "/") }
func filepathFromSlash(p string) string { return strings.ReplaceAll(p, "/", "\\") }

// buildWSLShellCommand renders the command executed inside the distro by a
// login + interactive bash shell:
//
//	export PATH="$HOME/.local/bin:$PATH"; cd '<dir>' && 'omp' '--model' 'p/m' || { echo; echo '...'; read _; }
//
// Every user-supplied value is wrapped in shSingleQuote so it can never break
// out of the shell, whatever characters it contains (spaces, backslashes,
// quotes, shell metacharacters). The explicit PATH export covers the canonical
// ~/.local/bin install location even when the user's rc files never source it.
// When the working directory is missing or OMP exits with an error, the
// console stays open so the user can read the message instead of watching a
// window flash shut.
func buildWSLShellCommand(ompCommand string, args []string, workingDir string) string {
	var sb strings.Builder
	sb.WriteString(`export PATH="$HOME/.local/bin:$PATH"; `)
	if workingDir != "" {
		sb.WriteString("cd " + shSingleQuote(workingDir) + " && ")
	}
	sb.WriteString(shSingleQuote(ompCommand))
	for _, arg := range args {
		sb.WriteString(" " + shSingleQuote(arg))
	}
	sb.WriteString(` || { echo; echo 'OMP 启动失败，按回车键关闭窗口...'; read _; }`)
	return sb.String()
}

// shSingleQuote wraps value in single quotes with embedded single quotes
// escaped the POSIX way ('\''), making the result safe to embed verbatim in
// any shell command.
func shSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func validateWSLInput(ompCommand string, args []string, distro string, workingDir string) error {
	ompCommand = strings.TrimSpace(ompCommand)
	if ompCommand == "" {
		return errors.New("OMP 命令不能为空")
	}
	if strings.IndexFunc(ompCommand, isControlChar) >= 0 {
		return errors.New("OMP 命令不能包含控制字符")
	}
	if len(args) == 0 {
		return errors.New("OMP 启动参数不能为空")
	}
	for _, a := range args {
		if strings.IndexFunc(a, isControlChar) >= 0 {
			return errors.New("OMP 启动参数不能包含控制字符")
		}
	}
	if strings.IndexFunc(distro, isControlChar) >= 0 {
		return errors.New("WSL 发行版名称不能包含控制字符")
	}
	if strings.IndexFunc(workingDir, isControlChar) >= 0 {
		return errors.New("工作目录不能包含控制字符")
	}
	return nil
}

func isControlChar(r rune) bool {
	return r < 0x20 || r == 0x7f
}
