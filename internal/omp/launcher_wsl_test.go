package omp

import (
	"strings"
	"testing"
)

func TestWindowsToWSLPath(t *testing.T) {
	tests := []struct{ in, want string }{
		{"E:\\woker\\project", "/mnt/e/woker/project"},
		{"E:/woker/project", "/mnt/e/woker/project"},
		{"C:\\", "/mnt/c"},
		{" e:\\x ", "/mnt/e/x"},
		{"relative\\dir", "relative/dir"},
		{"\\\\server\\share", "//server/share"},
	}
	for _, test := range tests {
		if got := WindowsToWSLPath(test.in); got != test.want {
			t.Fatalf("WindowsToWSLPath(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}

func TestWSLToWindowsPath(t *testing.T) {
	tests := []struct{ in, want string }{
		{"/mnt/e/woker", "E:\\woker"},
		{"/mnt/c", "C:"},
		{"/home/user", ""},
		{"/", ""},
		{"/mnt", ""},
		{"", ""},
	}
	for _, test := range tests {
		if got := WSLToWindowsPath(test.in); got != test.want {
			t.Fatalf("WSLToWindowsPath(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}

func TestWSLUNCToWSLPath(t *testing.T) {
	tests := []struct{ in, distro, want string }{
		{`\\wsl.localhost\Ubuntu\home\admin\.omp\agent\sessions\p\s.jsonl`, "Ubuntu", "/home/admin/.omp/agent/sessions/p/s.jsonl"},
		{`\\wsl.localhost\Ubuntu\home\admin\x`, "ubuntu", "/home/admin/x"},
		{`\\wsl$\Ubuntu\home\admin\x`, "Ubuntu", "/home/admin/x"},
		{`\\wsl.localhost\Debian\home\admin\x`, "Ubuntu", ""},
		{"E:\\woker\\x", "Ubuntu", ""},
		{`\\\\server\\share\\x`, "Ubuntu", ""},
		{`\\wsl.localhost\Ubuntu\x`, "", ""},
		{"", "Ubuntu", ""},
	}
	for _, test := range tests {
		if got := WSLUNCToWSLPath(test.in, test.distro); got != test.want {
			t.Fatalf("WSLUNCToWSLPath(%q, %q) = %q, want %q", test.in, test.distro, got, test.want)
		}
	}
}

func TestShSingleQuote(t *testing.T) {
	tests := []struct{ in, want string }{
		{"plain", "'plain'"},
		{"a b", "'a b'"},
		{"it's", `'it'\''s'`},
		{"C:\\x", `'C:\x'`},
		{"$(rm -rf /); echo hi", "'$(rm -rf /); echo hi'"},
	}
	for _, test := range tests {
		if got := shSingleQuote(test.in); got != test.want {
			t.Fatalf("shSingleQuote(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}

func TestBuildWSLShellCommand(t *testing.T) {
	got := buildWSLShellCommand("omp", []string{"--model", "deepseek/deepseek-v4-flash"}, "/mnt/e/woker/project")
	want := `export PATH="$HOME/.local/bin:$PATH"; cd '/mnt/e/woker/project' && 'omp' '--model' 'deepseek/deepseek-v4-flash' || { echo; echo 'OMP 启动失败，按回车键关闭窗口...'; read _; }`
	if got != want {
		t.Fatalf("command = %q\nwant %q", got, want)
	}
}

func TestBuildWSLShellCommandEscapesUnsafeValues(t *testing.T) {
	got := buildWSLShellCommand("omp", []string{"--resume", "C:\\Users\\me\\My Project\\s.jsonl"}, "/home/me/my dir")
	want := `export PATH="$HOME/.local/bin:$PATH"; cd '/home/me/my dir' && 'omp' '--resume' 'C:\Users\me\My Project\s.jsonl' || { echo; echo 'OMP 启动失败，按回车键关闭窗口...'; read _; }`
	if got != want {
		t.Fatalf("command = %q\nwant %q", got, want)
	}
}

func TestBuildWSLShellCommandWithoutWorkingDir(t *testing.T) {
	got := buildWSLShellCommand("omp", []string{"--model", "p/m"}, "")
	if strings.Contains(got, "cd ") {
		t.Fatalf("unexpected cd in command: %s", got)
	}
	if !strings.Contains(got, "'omp' '--model' 'p/m'") {
		t.Fatalf("payload missing: %s", got)
	}
}

func TestValidateWSLInput(t *testing.T) {
	if err := validateWSLInput("omp", []string{"--model", "p/m"}, "Ubuntu", "/home/x"); err != nil {
		t.Fatal(err)
	}
	if err := validateWSLInput("omp", []string{"--resume", "C:\\x\\y z.jsonl"}, "", ""); err != nil {
		t.Fatal(err)
	}
	if err := validateWSLInput("", []string{"--model", "p/m"}, "Ubuntu", "/x"); err == nil {
		t.Fatal("empty command accepted")
	}
	if err := validateWSLInput("omp", nil, "Ubuntu", "/x"); err == nil {
		t.Fatal("empty args accepted")
	}
	if err := validateWSLInput("omp", []string{"--model", "p/m"}, "U\nuntu", "/x"); err == nil {
		t.Fatal("control char in distro accepted")
	}
	if err := validateWSLInput("omp", []string{"--model", "p/m"}, "Ubuntu", "/x\u0001"); err == nil {
		t.Fatal("control char in working dir accepted")
	}
}
