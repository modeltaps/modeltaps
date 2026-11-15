package channel

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
)

type kindKey struct{}

// WithKind attaches the notice kind to a send, for notifiers that pass it on.
func WithKind(ctx context.Context, kind string) context.Context {
	return context.WithValue(ctx, kindKey{}, kind)
}

// KindFrom returns the notice kind attached with WithKind, or "".
func KindFrom(ctx context.Context) string {
	kind, _ := ctx.Value(kindKey{}).(string)
	return kind
}

// webhookClient posts notices; the request context bounds tests, this bounds the rest.
var webhookClient = &http.Client{Timeout: 15 * time.Second}

// postJSON posts payload as JSON and treats any 2xx answer as delivered.
func postJSON(ctx context.Context, url string, payload any, headers map[string]string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return postBody(ctx, url, body, headers)
}

func postBody(ctx context.Context, url string, body []byte, headers map[string]string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Modeltaps/"+config.Version)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := webhookClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return nil
}

// Webhook posts every notice as JSON to a URL of the admin's choice:
//
//	{"kind": "low_balance", "title": "...", "message": "...", "site": "...", "time": 1731650000}
//
// With a secret, the X-Modeltaps-Signature header carries "sha256=" + the hex HMAC-SHA256 of the
// body, so the receiver can check the notice came from this platform.
type Webhook struct {
	URL    string
	Secret string
}

func NewWebhook(url, secret string) *Webhook { return &Webhook{URL: url, Secret: secret} }

func (w *Webhook) Name() string { return "Webhook" }

type webhookPayload struct {
	Kind    string `json:"kind"`
	Title   string `json:"title"`
	Message string `json:"message"`
	Site    string `json:"site"`
	Time    int64  `json:"time"`
}

// WebhookSignature is the X-Modeltaps-Signature value of body.
func WebhookSignature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func (w *Webhook) Send(ctx context.Context, title, message string) error {
	body, err := json.Marshal(webhookPayload{
		Kind: KindFrom(ctx), Title: title, Message: message, Site: config.SystemName, Time: time.Now().Unix(),
	})
	if err != nil {
		return err
	}
	headers := map[string]string{}
	if w.Secret != "" {
		headers["X-Modeltaps-Signature"] = WebhookSignature(w.Secret, body)
	}
	return postBody(ctx, w.URL, body, headers)
}

// Slack posts notices to a Slack incoming webhook.
type Slack struct {
	Webhook string
}

func NewSlack(webhook string) *Slack { return &Slack{Webhook: webhook} }

func (s *Slack) Name() string { return "Slack" }

func (s *Slack) Send(ctx context.Context, title, message string) error {
	return postJSON(ctx, s.Webhook, map[string]string{"text": fmt.Sprintf("*%s*\n%s", title, message)}, nil)
}

// discordMaxContent is Discord's message length limit.
const discordMaxContent = 2000

// Discord posts notices to a Discord channel webhook.
type Discord struct {
	Webhook string
}

func NewDiscord(webhook string) *Discord { return &Discord{Webhook: webhook} }

func (d *Discord) Name() string { return "Discord" }

func (d *Discord) Send(ctx context.Context, title, message string) error {
	content := []rune(fmt.Sprintf("**%s**\n%s", title, message))
	if len(content) > discordMaxContent {
		content = append(content[:discordMaxContent-1], '…')
	}
	return postJSON(ctx, d.Webhook, map[string]string{"content": string(content)}, nil)
}
