package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestModelSendsShortOpenAIRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" || request.Header.Get("Authorization") != "Bearer secret" || request.Header.Get("X-Tenant") != "one" {
			t.Errorf("request path=%q headers=%v", request.URL.Path, request.Header)
		}
		var payload struct {
			Model     string `json:"model"`
			MaxTokens int    `json:"max_tokens"`
			Messages  []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Model != "model" || payload.MaxTokens != 16 || len(payload.Messages) != 1 || payload.Messages[0].Content != "Hi" {
			t.Errorf("payload=%#v", payload)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"OK"}}]}`))
	}))
	defer server.Close()
	result, err := TestModel(context.Background(), Config{ID: "gateway", BaseURL: server.URL + "/v1", API: "openai-completions", Headers: map[string]string{"X-Tenant": "one"}}, ModelInfo{ID: "model"}, "secret", ModelTestOptions{})
	if err != nil || !result.OK || len(result.Lines) != 2 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestModelUsesModelProtocolAndReturnsUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/messages" || request.Header.Get("x-api-key") != "secret" || request.Header.Get("anthropic-version") == "" {
			t.Errorf("request path=%q headers=%v", request.URL.Path, request.Header)
		}
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"error":"bad model"}`))
	}))
	defer server.Close()
	_, err := TestModel(context.Background(), Config{BaseURL: server.URL + "/v1", API: "openai-completions"}, ModelInfo{ID: "model", API: "anthropic-messages"}, "secret", ModelTestOptions{})
	if err == nil || !strings.Contains(err.Error(), "400 Bad Request") || !strings.Contains(err.Error(), "bad model") {
		t.Fatalf("error=%v", err)
	}
}
