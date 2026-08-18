package main

import (
	"os"
	"path/filepath"
	"testing"
)

func utf16LEBOM(s string) []byte {
	buf := []byte{0xFF, 0xFE}
	for _, r := range s {
		u16 := uint16(r)
		buf = append(buf, byte(u16&0xFF), byte(u16>>8))
	}
	return buf
}

func TestParseWSLDistroVerboseEnglish(t *testing.T) {
	table := "  NAME                   STATE           VERSION\r\n" +
		"* Ubuntu                 Running         2\r\n" +
		"  Debian                 Stopped         1\r\n" +
		"  Ubuntu-22.04           Stopped         2\r\n" +
		"  docker-desktop         Stopped         2\r\n"
	got := parseWSLDistroVerbose(utf16LEBOM(table))
	if len(got) != 4 {
		t.Fatalf("got %d distros, want 4: %#v", len(got), got)
	}
	if got[0].ID != "Ubuntu" || !got[0].IsDefault || got[0].Version != 2 {
		t.Fatalf("distro[0] = %#v", got[0])
	}
	if got[3].ID != "docker-desktop" || got[3].Version != 2 {
		t.Fatalf("distro[3] = %#v", got[3])
	}
}

func TestParseWSLDistroVerboseChineseHeader(t *testing.T) {
	// zh-CN localization: 名称 状态 版本
	table := "\u0020\u0020\u540d\u79f0\u0020\u0020\u0020\u0020\u0020\u0020\u0020\u0020\u0020\u0020\u0020\u72b6\u6001\u0020\u0020\u0020\u0020\u0020\u0020\u0020\u0020\u0020\u7248\u672c\r\n" +
		"* Ubuntu-26.04          \u6b63\u5728\u8fd0\u884c         2\r\n" +
		"  docker-desktop        \u5df2\u505c\u6b62           2\r\n"
	got := parseWSLDistroVerbose(utf16LEBOM(table))
	if len(got) != 2 {
		t.Fatalf("got %d distros, want 2 (header must be skipped): %#v", len(got), got)
	}
	if got[0].ID != "Ubuntu-26.04" || !got[0].IsDefault || got[0].Version != 2 {
		t.Fatalf("distro[0] = %#v", got[0])
	}
	if got[1].ID != "docker-desktop" {
		t.Fatalf("distro[1] = %#v", got[1])
	}
}

func TestParseWSLDistroVerboseEmptyAndGarbage(t *testing.T) {
	if got := parseWSLDistroVerbose(nil); len(got) != 0 {
		t.Fatalf("nil input parsed to %#v", got)
	}
	if got := parseWSLDistroVerbose([]byte{0xFF, 0xFE, 0x00}); len(got) != 0 {
		t.Fatalf("odd-length input parsed to %#v", got)
	}
	if got := parseWSLDistroVerbose(utf16LEBOM("NAME STATE VERSION\r\n")); len(got) != 0 {
		t.Fatalf("header-only parsed to %#v", got)
	}
}

func TestParseWSLDistroOutput(t *testing.T) {
	got := parseWSLDistroOutput(utf16LEBOM("Ubuntu\r\nDebian\r\n"))
	if len(got) != 2 || got[0] != "Ubuntu" || got[1] != "Debian" {
		t.Fatalf("got %#v", got)
	}
}

func TestResolveNativePathsDetectsModelsFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
	models := filepath.Join(root, ".omp", "agent", "models.yml")
	if err := os.MkdirAll(filepath.Dir(models), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(models, []byte("providers: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := new(App).ResolveNativePaths()
	if err != nil {
		t.Fatal(err)
	}
	if !result.Detected {
		t.Fatalf("expected detected=true, got %#v", result)
	}
	if result.CustomPaths.OMPModelsPath != models {
		t.Fatalf("models path = %q, want %q", result.CustomPaths.OMPModelsPath, models)
	}
	if result.Mode != "native" {
		t.Fatalf("mode = %q, want native", result.Mode)
	}
}
