package provider

import (
	"context"
	"io"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestResolveAPIKeyEnvironmentAndCommandValues(t *testing.T) {
	lookup := func(name string) (string, bool) {
		values := map[string]string{"TOKEN": " environment-secret ", "COMMAND": " !read-secret "}
		value, found := values[name]
		return value, found
	}
	for _, test := range []struct {
		value, key string
		command    bool
	}{
		{" TOKEN ", "environment-secret", false}, {" literal ", "literal", false}, {"COMMAND", "!read-secret", true},
	} {
		key, command := ResolveAPIKey(test.value, lookup)
		if key != test.key || command != test.command {
			t.Fatalf("ResolveAPIKey(%q) = %q, %v", test.value, key, command)
		}
	}
}

func TestDiscoverModelsRequestPagingHeadersAndEnrichment(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		page := requests.Add(1)
		if request.URL.Path != "/v1/models" || request.URL.Query().Get("tenant") != "a" {
			t.Errorf("URL = %s", request.URL.String())
		}
		if request.Header.Get("Authorization") != "Custom secret" || request.Header.Get("X-Command") != "" {
			t.Errorf("headers = %v", request.Header)
		}
		if page == 1 {
			_, _ = writer.Write([]byte(`{"data":[{"id":"first"},{"id":"duplicate"}],"has_more":true,"last_id":"cursor-1"}`))
			return
		}
		if request.URL.Query().Get("after") != "cursor-1" {
			t.Errorf("after = %q", request.URL.Query().Get("after"))
		}
		_, _ = writer.Write([]byte(`{"data":[{"id":"duplicate","display_name":"Enriched","reasoning":false,"context_length":100,"max_tokens":20},{"id":"last"}],"has_more":false}`))
	}))
	defer server.Close()

	result, err := DiscoverModels(context.Background(), Config{BaseURL: server.URL + "/v1?tenant=a", Headers: map[string]string{
		"Authorization": "Custom secret", "X-Command": "!load-header",
	}}, "bearer-secret", DiscoveryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{result.Models[0].ID, result.Models[1].ID, result.Models[2].ID}
	if !reflect.DeepEqual(ids, []string{"first", "duplicate", "last"}) {
		t.Fatalf("ids = %v", ids)
	}
	duplicate := result.Models[1]
	if duplicate.Name != "Enriched" || duplicate.Reasoning == nil || *duplicate.Reasoning || duplicate.ContextWindow != 100 || duplicate.MaxTokens != 20 {
		t.Fatalf("duplicate = %+v", duplicate)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "X-Command") {
		t.Fatalf("warnings = %v", result.Warnings)
	}
}

func TestDiscoverModelsGeminiPaginationPreservesQuery(t *testing.T) {
	var page int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		page++
		if request.URL.Query().Get("key") != "fixed" {
			t.Errorf("query = %s", request.URL.RawQuery)
		}
		if page == 1 {
			_, _ = writer.Write([]byte(`{"models":[{"name":"models/gemini-1","supportedGenerationMethods":["generateContent"]}],"nextPageToken":"next"}`))
			return
		}
		if request.URL.Query().Get("pageToken") != "next" {
			t.Errorf("pageToken = %q", request.URL.Query().Get("pageToken"))
		}
		_, _ = writer.Write([]byte(`{"models":[{"name":"models/gemini-2"}]}`))
	}))
	defer server.Close()
	result, err := DiscoverModels(context.Background(), Config{BaseURL: server.URL + "?key=fixed"}, "secret", DiscoveryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{result.Models[0].ID, result.Models[1].ID}; !reflect.DeepEqual(got, []string{"gemini-1", "gemini-2"}) {
		t.Fatalf("ids = %v", got)
	}
}

func TestDiscoverModelsAllOrNothingFailures(t *testing.T) {
	tests := []struct {
		name     string
		maxPages int
		contains string
		handler  http.HandlerFunc
	}{
		{"cycle", 0, "循环", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"data":[{"id":"one"}],"has_more":true,"last_id":"same"}`))
		}},
		{"page cap", 2, "2 页上限", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"data":[{"id":"one"}],"has_more":true,"last_id":"` + r.URL.Query().Get("after") + `x"}`))
		}},
		{"middle failure", 0, "502", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("after") == "first" {
				w.WriteHeader(502)
				_, _ = w.Write([]byte("gateway failed"))
				return
			}
			_, _ = w.Write([]byte(`{"data":[{"id":"one"}],"has_more":true,"last_id":"first"}`))
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(test.handler)
			defer server.Close()
			result, err := DiscoverModels(context.Background(), Config{BaseURL: server.URL}, "", DiscoveryOptions{MaxPages: test.maxPages})
			if err == nil || !strings.Contains(err.Error(), test.contains) || len(result.Models) != 0 {
				t.Fatalf("result=%+v error=%v", result, err)
			}
		})
	}
}

func TestDiscoverModelsCancellationClosesRequest(t *testing.T) {
	started, done := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done(); close(done) }))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	errorsOut := make(chan error, 1)
	go func() {
		_, err := DiscoverModels(ctx, Config{BaseURL: server.URL}, "", DiscoveryOptions{PageTimeout: time.Second})
		errorsOut <- err
	}()
	<-started
	cancel()
	if err := <-errorsOut; !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("request context not canceled")
	}
}

func TestReadDiscoveryResponseHTML404GivesHint(t *testing.T) {
	response := &http.Response{
		StatusCode: http.StatusNotFound,
		Status:     "404 Not Found",
		Body:       io.NopCloser(strings.NewReader("<!DOCTYPE html><html><body><h1>Not Found</h1></body></html>")),
	}
	_, err := readDiscoveryResponse(response)
	if err == nil {
		t.Fatal("expected error for 404 HTML response")
	}
	message := err.Error()
	if strings.Contains(message, "<html") || strings.Contains(message, "<body") {
		t.Fatalf("HTML body leaked into error: %q", message)
	}
	if !strings.Contains(message, "不是 OpenAI 兼容 API 端点") {
		t.Fatalf("expected fix-oriented hint, got %q", message)
	}
}

func TestReadDiscoveryResponseTruncatesLongJSONError(t *testing.T) {
	long := strings.Repeat("x", 1024)
	response := &http.Response{
		StatusCode: http.StatusBadRequest,
		Status:     "400 Bad Request",
		Body:       io.NopCloser(strings.NewReader(`{"error":"` + long + `"}`)),
	}
	_, err := readDiscoveryResponse(response)
	if err == nil {
		t.Fatal("expected error")
	}
	if len(err.Error()) > 400 {
		t.Fatalf("error too long (%d bytes): %q", len(err.Error()), err.Error())
	}
}
