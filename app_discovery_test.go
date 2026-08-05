package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"ompswitch/internal/config"
	"ompswitch/internal/paths"
	"ompswitch/internal/provider"
)

func discoveryTestApp(t *testing.T, providers []provider.Config) *App {
	t.Helper()
	root := t.TempDir()
	appPaths := paths.AppPaths{
		OMPSwitchConfigPath: filepath.Join(root, ".ompswitch", "config.json"),
		OMPModelsPath:       filepath.Join(root, ".omp", "agent", "models.yml"),
		OMPConfigPath:       filepath.Join(root, ".omp", "agent", "config.yml"),
		BackupDir:           filepath.Join(root, ".ompswitch", "backups"),
	}
	service := config.NewService(appPaths)
	cfg := config.SwitchConfig{Providers: providers, ModelRoles: map[string]string{}}
	if len(providers) > 0 {
		cfg.SelectedProviderID = providers[0].ID
	}
	if err := service.SaveAppOnly(cfg); err != nil {
		t.Fatal(err)
	}
	return &App{
		ctx: context.Background(), service: service, paths: appPaths,
		discoveryByRequest: map[string]context.CancelFunc{}, discoveryByProvider: map[string]string{},
	}
}

func TestFetchModelsConcurrencyCancellationAndCleanup(t *testing.T) {
	started := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		started <- request.URL.Query().Get("provider")
		<-request.Context().Done()
	}))
	defer server.Close()
	providerConfig := func(id string) provider.Config {
		return provider.Config{ID: id, Name: id, BaseURL: server.URL + "?provider=" + id, APIKey: "literal", API: "openai-completions", Headers: map[string]string{}, CustomHeaders: map[string]string{}}
	}
	app := discoveryTestApp(t, []provider.Config{providerConfig("one"), providerConfig("two")})

	type outcome struct {
		result provider.DiscoveryResult
		err    error
	}
	outcomes := make(chan outcome, 2)
	go func() { result, err := app.FetchModels("one", "request-one"); outcomes <- outcome{result, err} }()
	if got := <-started; got != "one" {
		t.Fatalf("first provider = %q", got)
	}
	if _, err := app.FetchModels("one", "request-two"); err == nil || !strings.Contains(err.Error(), "Provider 正在") {
		t.Fatalf("same-provider error = %v", err)
	}
	if _, err := app.FetchModels("two", "request-one"); err == nil || !strings.Contains(err.Error(), "请求 ID 正在") {
		t.Fatalf("duplicate request error = %v", err)
	}

	go func() { result, err := app.FetchModels("two", "request-two"); outcomes <- outcome{result, err} }()
	if got := <-started; got != "two" {
		t.Fatalf("second provider = %q", got)
	}
	if err := app.CancelModelDiscovery("request-one"); err != nil {
		t.Fatal(err)
	}
	if err := app.CancelModelDiscovery("request-two"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		outcome := <-outcomes
		if outcome.err == nil || outcome.err.Error() != "已取消获取模型" || len(outcome.result.Models) != 0 {
			t.Fatalf("outcome = %+v", outcome)
		}
	}
	app.discoveryMu.Lock()
	defer app.discoveryMu.Unlock()
	if len(app.discoveryByRequest) != 0 || len(app.discoveryByProvider) != 0 {
		t.Fatalf("registries not cleaned: %v %v", app.discoveryByRequest, app.discoveryByProvider)
	}
}

func TestImportDiscoveredModelsIsStaleSafeAndEnrichesUnknownOnly(t *testing.T) {
	falseValue, trueValue := false, true
	stored := provider.Config{
		ID: "provider", Name: "Provider", BaseURL: "https://example.com/v1", APIKey: "literal", API: "openai-completions",
		Headers: map[string]string{}, CustomHeaders: map[string]string{}, SelectedModelID: "manual",
		Models: []provider.ModelInfo{
			{ID: "manual", Name: "Manual", Reasoning: &falseValue, ContextWindow: 999},
			{ID: "added-after-fetch", Name: "Added after fetch"},
		},
	}
	app := discoveryTestApp(t, []provider.Config{stored})
	state, err := app.ImportDiscoveredModels("provider", []provider.ModelInfo{
		{ID: "manual", Name: "Upstream", Reasoning: &trueValue, ContextWindow: 100, MaxTokens: 20},
		{ID: "new", Name: "New"},
	})
	if err != nil {
		t.Fatal(err)
	}
	models := state.Providers[0].Models
	ids := []string{models[0].ID, models[1].ID, models[2].ID}
	if !reflect.DeepEqual(ids, []string{"manual", "added-after-fetch", "new"}) {
		t.Fatalf("ids = %v", ids)
	}
	if models[0].Name != "Manual" || models[0].Reasoning == nil || *models[0].Reasoning || models[0].ContextWindow != 999 || models[0].MaxTokens != 20 {
		t.Fatalf("manual model = %+v", models[0])
	}
	if state.Providers[0].APIKey != "" || !state.Providers[0].HasAPIKey {
		t.Fatalf("secret boundary = %+v", state.Providers[0])
	}
	loaded, err := app.service.Load()
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := loaded.ProviderByID("provider")
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{persisted.Models[0].ID, persisted.Models[1].ID, persisted.Models[2].ID}; !reflect.DeepEqual(got, ids) {
		t.Fatalf("persisted ids = %v", got)
	}
}

func TestFetchModelsRejectsCommandAPIKeyBeforeRequestAndCleansRegistry(t *testing.T) {
	requested := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requested <- struct{}{}
	}))
	defer server.Close()
	app := discoveryTestApp(t, []provider.Config{{
		ID: "command", Name: "Command", BaseURL: server.URL, APIKey: "!load-secret", API: "openai-completions",
		Headers: map[string]string{}, CustomHeaders: map[string]string{},
	}})
	if _, err := app.FetchModels("command", "command-request"); err == nil || !strings.Contains(err.Error(), "手工添加模型") {
		t.Fatalf("error = %v", err)
	}
	select {
	case <-requested:
		t.Fatal("command-valued key made an HTTP request")
	default:
	}
	app.discoveryMu.Lock()
	defer app.discoveryMu.Unlock()
	if len(app.discoveryByRequest) != 0 || len(app.discoveryByProvider) != 0 {
		t.Fatalf("registries not cleaned: %v %v", app.discoveryByRequest, app.discoveryByProvider)
	}
}

func TestCancelModelDiscoveryUnknownRequest(t *testing.T) {
	app := discoveryTestApp(t, nil)
	if err := app.CancelModelDiscovery("missing"); err == nil {
		t.Fatal("expected unknown request error")
	}
}
