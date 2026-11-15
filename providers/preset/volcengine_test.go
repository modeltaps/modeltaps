package preset

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

// The expected signature was computed apart, following Volcengine's signing guide.
func TestVolcengineSignature(t *testing.T) {
	now := time.Date(2026, 11, 15, 8, 0, 0, 0, time.UTC)
	req, err := volcengineBilling.request("AKTEST", "SKTEST", now)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != http.MethodPost || req.URL.String() != "https://open.volcengineapi.com/?Action=QueryBalanceAcct&Version=2022-01-01" {
		t.Errorf("request = %s %s", req.Method, req.URL)
	}
	want := "HMAC-SHA256 Credential=AKTEST/20261115/cn-north-1/billing/request, " +
		"SignedHeaders=content-type;host;x-content-sha256;x-date, " +
		"Signature=c08d8b8ee570b63c080a12d22feddde62e71430df5d1a725b50ee3d559f4ecd1"
	if got := req.Header.Get("Authorization"); got != want {
		t.Errorf("Authorization =\n %s\nwant\n %s", got, want)
	}
	if req.Header.Get("X-Date") != "20261115T080000Z" {
		t.Errorf("X-Date = %q", req.Header.Get("X-Date"))
	}
}

func TestVolcengineKeyAndBalance(t *testing.T) {
	manageKey := " AKX | SKX "
	channel := &model.Channel{Type: config.ChannelTypeVolcengine, Key: "ark-key", ManageKey: &manageKey}
	p := ProviderFactory{Preset: Presets[config.ChannelTypeVolcengine]}.Create(channel).(*Provider)
	if p.Channel.Key != "ark-key" || len(p.extraKeys) != 2 || p.extraKeys[1] != "SKX" {
		t.Fatalf("key = %q extra = %v", p.Channel.Key, p.extraKeys)
	}
	if !p.CanQueryBalance() {
		t.Error("Volcengine reads a balance with the management key")
	}

	var gotAuth string
	oldHTTP := volcHTTP
	volcHTTP = func(req *http.Request) (*http.Response, error) {
		gotAuth = req.Header.Get("Authorization")
		body := `{"ResponseMetadata":{"Action":"QueryBalanceAcct"},"Result":{"AccountID":1,"AvailableBalance":"123.45","CashBalance":"123.45"}}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	}
	t.Cleanup(func() { volcHTTP = oldHTTP })
	got, err := volcengineBilling.balance(p)
	if err != nil || got != 123.45 {
		t.Fatalf("balance = %v, %v", got, err)
	}
	if !strings.Contains(gotAuth, "Credential=AKX/") {
		t.Errorf("signed with %q", gotAuth)
	}

	volcHTTP = func(*http.Request) (*http.Response, error) {
		body := `{"ResponseMetadata":{"Error":{"Code":"InvalidCredential","Message":"bad key"}}}`
		return &http.Response{StatusCode: 400, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	}
	if _, err := volcengineBilling.balance(p); err == nil || !strings.Contains(err.Error(), "InvalidCredential") {
		t.Errorf("error = %v", err)
	}

	only := ProviderFactory{Preset: Presets[config.ChannelTypeVolcengine]}.Create(&model.Channel{Key: "ark-key"}).(*Provider)
	if _, err := volcengineBilling.balance(only); err == nil || !strings.Contains(err.Error(), "AccessKey") {
		t.Errorf("an API key alone should explain what is missing, got %v", err)
	}
	byteplus := ProviderFactory{Preset: Presets[config.ChannelTypeBytePlus]}.Create(&model.Channel{}).(*Provider)
	if byteplus.CanQueryBalance() {
		t.Error("BytePlus has no balance yet")
	}
}
