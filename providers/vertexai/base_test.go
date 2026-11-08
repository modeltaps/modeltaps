package vertexai

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/vertexai/category"
	"github.com/modeltaps/modeltaps/types"
)

func TestVertexHost(t *testing.T) {
	cases := map[string]string{
		"global":       "aiplatform.googleapis.com",
		"us":           "aiplatform.us.rep.googleapis.com",
		"eu":           "aiplatform.eu.rep.googleapis.com",
		"us-east5":     "us-east5-aiplatform.googleapis.com",
		"europe-west1": "europe-west1-aiplatform.googleapis.com",
	}
	for region, want := range cases {
		if got := vertexHost(region); got != want {
			t.Errorf("vertexHost(%q) = %q, want %q", region, got, want)
		}
	}
}

func providerWithBaseURL(region string, baseURL *string, cat *category.Category) *VertexAIProvider {
	p := &VertexAIProvider{Region: region, ProjectID: "proj-1", Category: cat}
	p.Channel = &model.Channel{Id: 1, BaseURL: baseURL}
	p.Config = getConfig()
	return p
}

func TestGetFullRequestURLDefaultTemplate(t *testing.T) {
	cases := []struct {
		name   string
		region string
		cat    *category.Category
		want   string
	}{
		{"gemini regional", "us-central1", category.CategoryMap["gemini"],
			"https://us-central1-aiplatform.googleapis.com/v1/projects/proj-1/locations/us-central1/publishers/google/models/m:act"},
		{"gemini global", "global", category.CategoryMap["gemini"],
			"https://aiplatform.googleapis.com/v1/projects/proj-1/locations/global/publishers/google/models/m:act"},
		{"claude regional", "us-east5", category.CategoryMap["claude"],
			"https://us-east5-aiplatform.googleapis.com/v1/projects/proj-1/locations/us-east5/publishers/anthropic/models/m:act"},
		{"claude us multi-region", "us", category.CategoryMap["claude"],
			"https://aiplatform.us.rep.googleapis.com/v1/projects/proj-1/locations/us/publishers/anthropic/models/m:act"},
		{"no category (imagen) defaults to google", "us-central1", nil,
			"https://us-central1-aiplatform.googleapis.com/v1/projects/proj-1/locations/us-central1/publishers/google/models/m:act"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := providerWithBaseURL(c.region, nil, c.cat)
			if got := p.GetFullRequestURL("m", "act"); got != c.want {
				t.Fatalf("got %q\nwant %q", got, c.want)
			}
		})
	}
}

func TestGetFullRequestURLCustomTemplates(t *testing.T) {
	current := "https://proxy.example.com/%s/p/%s/l/%s/pub/%s/m/%s/a/%s"
	legacy := "https://proxy.example.com/%saiplatform/p/%s/l/%s/m/%s/a/%s"
	tooFew := "https://proxy.example.com/%s/%s"

	p := providerWithBaseURL("us-east5", &current, category.CategoryMap["claude"])
	if got, want := p.GetFullRequestURL("m", "act"),
		"https://proxy.example.com/us-east5-aiplatform.googleapis.com/p/proj-1/l/us-east5/pub/anthropic/m/m/a/act"; got != want {
		t.Fatalf("current template: got %q, want %q", got, want)
	}

	// A template written before the publisher became a placeholder keeps serving Gemini,
	// and is refused for Claude because it can only address publishers/google.
	p = providerWithBaseURL("us-east5", &legacy, category.CategoryMap["gemini"])
	if got, want := p.GetFullRequestURL("m", "act"),
		"https://proxy.example.com/us-east5-aiplatform/p/proj-1/l/us-east5/m/m/a/act"; got != want {
		t.Fatalf("legacy template, gemini: got %q, want %q", got, want)
	}
	p = providerWithBaseURL("global", &legacy, nil)
	if got, want := p.GetFullRequestURL("m", "act"),
		"https://proxy.example.com/aiplatform/p/proj-1/l/global/m/m/a/act"; got != want {
		t.Fatalf("legacy template, global: got %q, want %q", got, want)
	}
	p = providerWithBaseURL("us-east5", &legacy, category.CategoryMap["claude"])
	if got := p.GetFullRequestURL("m", "act"); got != "" {
		t.Fatalf("legacy template, claude: got %q, want an empty URL", got)
	}

	// Anything else is refused instead of falling back to the built-in template, which would
	// send a proxied channel's traffic straight to Google.
	p = providerWithBaseURL("us-east5", &tooFew, category.CategoryMap["gemini"])
	if got := p.GetFullRequestURL("m", "act"); got != "" {
		t.Fatalf("invalid template: got %q, want an empty URL", got)
	}
}

func TestRequestErrorHandleSurvivesEmptyArrayBody(t *testing.T) {
	// A Google-style error list can be empty; reading element 0 used to panic.
	if err := errorBodyToError(t, `[]`, "proj-1"); err != nil {
		t.Fatalf("expected no error for an empty list, got %+v", err)
	}
	err := errorBodyToError(t, `[{"error":{"code":403,"message":"denied for proj-1","status":"PERMISSION_DENIED"}}]`, "proj-1")
	if err == nil || err.Message != "denied for xxxxx" || err.Type != "vertexai_error" || err.Param != "PERMISSION_DENIED" {
		t.Fatalf("unexpected error %+v", err)
	}
}

func errorBodyToError(t *testing.T, body, projectID string) *types.OpenAIError {
	t.Helper()
	resp := &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader(body))}
	return RequestErrorHandle(nil, projectID)(resp)
}
