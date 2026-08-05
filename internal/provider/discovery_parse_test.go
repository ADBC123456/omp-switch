package provider

import (
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestParseDiscoveryPageAcceptsBothStrictEnvelopes(t *testing.T) {
	falseValue := false
	tests := []struct {
		name string
		body string
		want []ModelInfo
	}{
		{
			name: "data aliases and explicit false",
			body: `{"data":[{"id":"chat","display_name":"Chat","reasoning":false,"input_token_limit":128,"output_token_limit":32,"capabilities":["messages"]}]}`,
			want: []ModelInfo{{ID: "chat", Name: "Chat", Reasoning: &falseValue, ContextWindow: 128, MaxTokens: 32}},
		},
		{
			name: "gemini",
			body: `{"models":[{"name":"models/gemini","displayName":"Gemini","inputTokenLimit":256,"outputTokenLimit":64,"supportedGenerationMethods":["generateContent"]}]}`,
			want: []ModelInfo{{ID: "gemini", Name: "Gemini", ContextWindow: 256, MaxTokens: 64}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page, err := parseDiscoveryPage([]byte(test.body))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(page.models, test.want) {
				t.Fatalf("models = %#v, want %#v", page.models, test.want)
			}
		})
	}
}

func TestParseDiscoveryPageFiltersOnlyExplicitNonChat(t *testing.T) {
	page, err := parseDiscoveryPage([]byte(`{"data":[
		{"id":"embed","type":"embedding"},
		{"id":"unknown","capabilities":["vendor-special"]},
		{"id":"mixed","capabilities":["chat","image"]},
		{"id":"normalized","task":"text_generation"}
	]}`))
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(page.models))
	for index, model := range page.models {
		ids[index] = model.ID
	}
	if !reflect.DeepEqual(ids, []string{"unknown", "mixed", "normalized"}) {
		t.Fatalf("ids = %v", ids)
	}

	gemini, err := parseDiscoveryPage([]byte(`{"models":[
		{"name":"models/text","supportedGenerationMethods":["generateContent"]},
		{"name":"models/embed","supportedGenerationMethods":["embedContent"]},
		{"name":"models/unknown"}
	]}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{gemini.models[0].ID, gemini.models[1].ID}; !reflect.DeepEqual(got, []string{"text", "unknown"}) {
		t.Fatalf("gemini ids = %v", got)
	}
}

func TestParseDiscoveryPageRejectsMalformedKnownFieldsAndCursors(t *testing.T) {
	bodies := []string{
		`[]`,
		`{"data":{}}`,
		`{"data":[{"id":1}]}`,
		`{"data":[{"id":"m","reasoning":"yes"}]}`,
		`{"data":[{"id":"m","context_window":0}]}`,
		`{"data":[{"id":"m","capabilities":[1]}]}`,
		`{"data":[{"id":"m"}],"has_more":"yes"}`,
		`{"data":[{"id":"m"}],"has_more":true}`,
		`{"models":[{"name":1}]}`,
		`{"models":[{"name":"m","supportedGenerationMethods":"generateContent"}]}`,
		`{"models":[{"name":"m"}],"nextPageToken":1}`,
		`{"data":[],"models":[]}`,
		`{"data":[]}`,
	}
	for _, body := range bodies {
		if page, err := parseDiscoveryPage([]byte(body)); err == nil {
			t.Fatalf("accepted %s as %+v", body, page)
		}
	}
}

func TestParseDiscoveryPageDuplicateEnrichmentKeepsFirstPosition(t *testing.T) {
	result, err := DiscoverModels(nil, Config{BaseURL: "http://example.invalid"}, "", DiscoveryOptions{Client: roundTripClient(func(requestBody string) string {
		return `{"data":[{"id":"a"},{"id":"b"},{"id":"a","name":"A","reasoning":true,"context_window":100,"max_output_tokens":20}]}`
	})})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Models) != 2 || result.Models[0].ID != "a" || result.Models[0].Name != "A" || result.Models[0].Reasoning == nil || !*result.Models[0].Reasoning {
		t.Fatalf("models = %+v", result.Models)
	}
}

func roundTripClient(body func(string) string) *http.Client {
	return &http.Client{Transport: roundTripper(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body(request.URL.String()))), Header: make(http.Header)}, nil
	})}
}

type roundTripper func(*http.Request) (*http.Response, error)

func (function roundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
