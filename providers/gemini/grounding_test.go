package gemini

import (
	"encoding/json"
	"strings"
	"testing"
)

const groundingFixture = `{
  "groundingMetadata": {
    "webSearchQueries": ["coffee near me"],
    "groundingChunks": [
      {"web": {"uri": "https://example.com/a", "title": "A"}},
      {"maps": {"uri": "https://maps.google.com/?cid=1", "title": "Cafe"}},
      {"retrievedContext": {"uri": "gs://bucket/doc", "title": "Doc"}}
    ],
    "groundingSupports": [
      {"segment": {"startIndex": 0, "endIndex": 5}, "groundingChunkIndices": [0, 1, 2]}
    ],
    "googleMapsWidgetContextToken": "widget-token",
    "searchEntryPoint": {"renderedContent": "<div></div>", "sdkBlob": "blob"}
  }
}`

func TestGroundingMetadataKeepsNonWebSources(t *testing.T) {
	var candidate GeminiChatCandidate
	if err := json.Unmarshal([]byte(groundingFixture), &candidate); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	annotations := candidate.ConvertGroundingToAnnotations()
	if len(annotations) != 3 {
		t.Fatalf("want 3 annotations (web, maps, retrievedContext), got %d", len(annotations))
	}
	wantURLs := []string{"https://example.com/a", "https://maps.google.com/?cid=1", "gs://bucket/doc"}
	for i, want := range wantURLs {
		if annotations[i].Url != want {
			t.Errorf("annotation %d url = %q, want %q", i, annotations[i].Url, want)
		}
	}

	md := formatGroundingMetadataAsMarkdown(candidate.GroundingMetadata)
	if !strings.Contains(md, "[Cafe](https://maps.google.com/?cid=1)") {
		t.Errorf("markdown is missing the maps source:\n%s", md)
	}

	// Fields the gateway does not interpret must survive a re-encode unchanged.
	out, err := json.Marshal(candidate.GroundingMetadata)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"googleMapsWidgetContextToken":"widget-token"`, `"sdkBlob":"blob"`, `"maps":`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("re-encoded metadata is missing %s: %s", want, out)
		}
	}
}

func TestGroundingAnnotationsNilMetadata(t *testing.T) {
	var candidate GeminiChatCandidate
	if got := candidate.ConvertGroundingToAnnotations(); got != nil {
		t.Errorf("want nil annotations without metadata, got %v", got)
	}
	if got := formatGroundingMetadataAsMarkdown(nil); got != "" {
		t.Errorf("want empty markdown without metadata, got %q", got)
	}
}
