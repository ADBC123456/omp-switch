package provider

import "testing"

func boolPointer(value bool) *bool { return &value }

func TestViewRedactsAPIKey(t *testing.T) {
	view := NewView(Config{
		ID: "provider", APIKey: "secret", Models: []ModelInfo{{ID: "model"}},
	})
	if view.APIKey != "" {
		t.Fatalf("APIKey leaked: %q", view.APIKey)
	}
	if !view.HasAPIKey {
		t.Fatal("HasAPIKey = false, want true")
	}
	if len(view.Models) != 1 || view.Models[0].ID != "model" {
		t.Fatalf("Models = %#v", view.Models)
	}
}

func TestNormalizeUsesProviderIDAsName(t *testing.T) {
	cfg := Normalize(Config{ID: " renamed ", Name: "stale"})
	if cfg.ID != "renamed" || cfg.Name != "renamed" {
		t.Fatalf("identity = ID %q, Name %q", cfg.ID, cfg.Name)
	}
}

func TestNormalizeModelsKeepsFirstAndReasoningTriState(t *testing.T) {
	models := NormalizeModels([]ModelInfo{
		{ID: " m1 ", Reasoning: boolPointer(false)},
		{ID: "m1", Name: "replacement", Reasoning: boolPointer(true)},
		{ID: "m2", Reasoning: nil},
		{ID: "  "},
	})
	if len(models) != 2 {
		t.Fatalf("len = %d, want 2: %#v", len(models), models)
	}
	if models[0].Name != "" {
		t.Fatalf("first duplicate was overwritten: %#v", models[0])
	}
	if models[0].Reasoning == nil || *models[0].Reasoning {
		t.Fatalf("explicit false not preserved: %#v", models[0].Reasoning)
	}
	if models[1].Reasoning != nil {
		t.Fatalf("unknown reasoning not preserved: %#v", models[1].Reasoning)
	}
}

func TestNormalizeSelectedModelFallback(t *testing.T) {
	cfg := Normalize(Config{SelectedModelID: "missing", Models: []ModelInfo{{ID: "first"}, {ID: "second"}}})
	if cfg.SelectedModelID != "first" {
		t.Fatalf("SelectedModelID = %q, want first", cfg.SelectedModelID)
	}
	cfg = Normalize(Config{SelectedModelID: "missing"})
	if cfg.SelectedModelID != "" {
		t.Fatalf("SelectedModelID = %q, want empty", cfg.SelectedModelID)
	}
}
