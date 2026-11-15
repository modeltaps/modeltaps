package channel

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebhookPostsSignedJSON(t *testing.T) {
	var got map[string]any
	var sig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		sig = r.Header.Get("X-Modeltaps-Signature")
		if sig != WebhookSignature("s3cret", body) {
			t.Errorf("bad signature %q", sig)
		}
		_ = json.Unmarshal(body, &got)
	}))
	defer srv.Close()

	err := NewWebhook(srv.URL, "s3cret").Send(WithKind(context.Background(), "low_balance"), "Low balance", "left: 1")
	if err != nil {
		t.Fatal(err)
	}
	if got["kind"] != "low_balance" || got["title"] != "Low balance" || got["message"] != "left: 1" {
		t.Fatalf("payload %v", got)
	}
	if !strings.HasPrefix(sig, "sha256=") {
		t.Fatalf("signature %q", sig)
	}
}

func TestWebhookReportsHTTPErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer srv.Close()
	if err := NewSlack(srv.URL).Send(context.Background(), "t", "m"); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("err = %v", err)
	}
}

func TestDiscordTrimsLongMessages(t *testing.T) {
	var content string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		content = body["content"]
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	if err := NewDiscord(srv.URL).Send(context.Background(), "t", strings.Repeat("x", 5000)); err != nil {
		t.Fatal(err)
	}
	if n := len([]rune(content)); n != discordMaxContent {
		t.Fatalf("content length %d", n)
	}
}
