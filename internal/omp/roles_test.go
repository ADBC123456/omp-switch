package omp

import (
	"reflect"
	"strings"
	"testing"

	"ompswitch/internal/provider"
)

func TestDecodeManagedRolesReadsOnlyManagedKeys(t *testing.T) {
	decoded, err := DecodeManagedRoles([]byte(`theme: dark
modelRoles:
  default: gateway/main
  task: gateway/worker:high
  experimental: other/custom
`))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"default": "gateway/main",
		"task":    "gateway/worker:high",
	}
	if !reflect.DeepEqual(decoded, want) {
		t.Fatalf("roles = %#v, want %#v", decoded, want)
	}
}

func TestDecodeManagedRolesAllowsEmptyAndMissingContainer(t *testing.T) {
	for _, input := range []string{"", "# comment only\n", "theme: dark\n"} {
		decoded, err := DecodeManagedRoles([]byte(input))
		if err != nil {
			t.Fatalf("input %q: %v", input, err)
		}
		if len(decoded) != 0 {
			t.Fatalf("input %q: roles = %#v", input, decoded)
		}
	}
}

func TestMergeManagedRolesPreservesUnknownConfigRolesOrderAndComments(t *testing.T) {
	input := []byte(`# top comment
theme: dark # theme comment
modelRoles:
  experimental: other/custom # unknown role
  default: old/model # replace me
  slow: old/slow # remove me
footer:
  enabled: true
`)
	merged, err := MergeManagedRoles(input, map[string]string{
		"default": "gateway/main",
		"task":    "gateway/worker:high",
		"unknown": "must/not/be-added",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(merged)
	for _, preserved := range []string{
		"# top comment",
		"theme: dark # theme comment",
		"experimental: other/custom # unknown role",
		"footer:",
		"enabled: true",
		"default: gateway/main",
		"task: gateway/worker:high",
	} {
		if !strings.Contains(text, preserved) {
			t.Fatalf("missing %q:\n%s", preserved, text)
		}
	}
	for _, removed := range []string{"slow:", "unknown:"} {
		if strings.Contains(text, removed) {
			t.Fatalf("unexpected %q:\n%s", removed, text)
		}
	}
	if strings.Index(text, "experimental:") > strings.Index(text, "default:") {
		t.Fatalf("existing role order changed:\n%s", text)
	}

	decoded, err := DecodeManagedRoles(merged)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"default": "gateway/main", "task": "gateway/worker:high"}
	if !reflect.DeepEqual(decoded, want) {
		t.Fatalf("roles = %#v, want %#v", decoded, want)
	}
}

func TestMergeManagedRolesCreatesMappingForEmptyInput(t *testing.T) {
	merged, err := MergeManagedRoles(nil, map[string]string{"vision": "gateway/vision"})
	if err != nil {
		t.Fatal(err)
	}
	if string(merged) != "modelRoles:\n  vision: gateway/vision\n" {
		t.Fatalf("merged = %q", merged)
	}
}

func TestManagedRolesRejectMalformedDocumentsAndContainers(t *testing.T) {
	inputs := []string{
		"[broken",
		"- modelRoles\n",
		"modelRoles: []\n",
	}
	for _, input := range inputs {
		t.Run(strings.ReplaceAll(input, "\n", " "), func(t *testing.T) {
			if _, err := DecodeManagedRoles([]byte(input)); err == nil {
				t.Fatal("DecodeManagedRoles succeeded, want error")
			}
			if _, err := MergeManagedRoles([]byte(input), map[string]string{"default": "gateway/main"}); err == nil {
				t.Fatal("MergeManagedRoles succeeded, want error")
			}
		})
	}

	if _, err := DecodeManagedRoles([]byte("modelRoles:\n  default: [gateway/main]\n")); err == nil {
		t.Fatal("DecodeManagedRoles accepted non-string managed role")
	}
}

func TestParseManagedSelectorPrefersFullRawMatchBeforeThinkingSuffix(t *testing.T) {
	providers := []provider.Config{{
		ID: "gateway",
		Models: []provider.ModelInfo{{ID: "team/model"}, {ID: "ending:low"}},
	}}
	providerID, modelID, thinking, ok := ParseManagedSelector("gateway/ending:low", providers)
	if !ok || providerID != "gateway" || modelID != "ending:low" || thinking != "" {
		t.Fatalf("full match = %q %q %q %v", providerID, modelID, thinking, ok)
	}
	providerID, modelID, thinking, ok = ParseManagedSelector("gateway/team/model:high", providers)
	if !ok || providerID != "gateway" || modelID != "team/model" || thinking != "high" {
		t.Fatalf("suffix match = %q %q %q %v", providerID, modelID, thinking, ok)
	}
}

func TestRewriteManagedSelectorsPreservesCustomValues(t *testing.T) {
	providers := []provider.Config{{ID: "old", Models: []provider.ModelInfo{{ID: "model"}}}}
	roles := map[string]string{"default": "old/model:high", "task": "custom/value"}
	changed := RewriteManagedSelectors(roles, providers, "old", "model", "new", "renamed")
	if !reflect.DeepEqual(changed, []string{"default"}) {
		t.Fatalf("changed = %#v", changed)
	}
	if roles["default"] != "new/renamed:high" || roles["task"] != "custom/value" {
		t.Fatalf("roles = %#v", roles)
	}
}
