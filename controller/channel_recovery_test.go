package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/notify"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"

	"go.uber.org/zap"
)

func (r *recordingNotifier) countContaining(part string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, title := range r.titles {
		if strings.Contains(title, part) {
			n++
		}
	}
	return n
}

func channelStatus(t *testing.T, id int) int {
	t.Helper()
	var got model.Channel
	if err := model.DB.First(&got, id).Error; err != nil {
		t.Fatal(err)
	}
	return got.Status
}

// A recovery re-enables an automatically disabled channel once and sends one notice; a second
// call (another node running the same check) changes nothing and stays quiet.
func TestRecoverChannelReEnablesOnceAndNotifies(t *testing.T) {
	setupChannelTestDB(t)
	notify.AddNotifiers(testNotifier)

	channel := &model.Channel{Name: "drained", Type: config.ChannelTypeOpenAI, Status: config.ChannelStatusAutoDisabled}
	if err := model.DB.Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	title := fmt.Sprintf("(#%d) has been re-enabled", channel.Id)
	before := testNotifier.countContaining(title)

	if !RecoverChannel(channel.Id, channel.Name, true) {
		t.Fatal("first recovery returned false, want true")
	}
	if got := channelStatus(t, channel.Id); got != config.ChannelStatusEnabled {
		t.Fatalf("status = %d after recovery, want enabled", got)
	}
	if got := testNotifier.countContaining(title) - before; got != 1 {
		t.Fatalf("recovery notices = %d, want 1", got)
	}

	if RecoverChannel(channel.Id, channel.Name, true) {
		t.Fatal("second recovery returned true, want false")
	}
	if got := testNotifier.countContaining(title) - before; got != 1 {
		t.Fatalf("recovery notices = %d after a repeated recovery, want still 1", got)
	}
}

func TestRecoverChannelLeavesManuallyDisabledAlone(t *testing.T) {
	setupChannelTestDB(t)
	notify.AddNotifiers(testNotifier)

	channel := &model.Channel{Name: "parked", Type: config.ChannelTypeOpenAI, Status: config.ChannelStatusManuallyDisabled}
	if err := model.DB.Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	before := testNotifier.count()
	if RecoverChannel(channel.Id, channel.Name, true) {
		t.Fatal("recovered a manually disabled channel")
	}
	if got := channelStatus(t, channel.Id); got != config.ChannelStatusManuallyDisabled {
		t.Fatalf("status = %d, want manually disabled", got)
	}
	if got := testNotifier.count() - before; got != 0 {
		t.Fatalf("notices = %d for a manually disabled channel, want 0", got)
	}
}

func TestRecoverChannelWithoutNotice(t *testing.T) {
	setupChannelTestDB(t)
	notify.AddNotifiers(testNotifier)

	channel := &model.Channel{Name: "quiet", Type: config.ChannelTypeOpenAI, Status: config.ChannelStatusAutoDisabled}
	if err := model.DB.Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	before := testNotifier.count()
	if !RecoverChannel(channel.Id, channel.Name, false) {
		t.Fatal("recovery returned false, want true")
	}
	if got := testNotifier.count() - before; got != 0 {
		t.Fatalf("notices = %d with sendNotify=false, want 0", got)
	}
}

// The scheduled recovery check re-enables a drained channel whose upstream works again and
// sends a re-enabled notice, since scheduled runs send no summary.
func TestScheduledRecoveryCheckNotifiesReEnabledChannel(t *testing.T) {
	setModelInfoModeSwitch(t, false)
	oldLogger := logger.Logger
	logger.Logger = zap.NewNop()
	t.Cleanup(func() { logger.Logger = oldLogger })
	requester.InitHttpClient()
	setupChannelTestDB(t)
	notify.AddNotifiers(testNotifier)

	oldEnable := config.AutomaticEnableChannelEnabled
	config.AutomaticEnableChannelEnabled = true
	t.Cleanup(func() { config.AutomaticEnableChannelEnabled = oldEnable })

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"c1","object":"chat.completion","model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	t.Cleanup(upstream.Close)

	baseURL := upstream.URL
	channel := &model.Channel{
		Type:      config.ChannelTypeOpenAI,
		Key:       "sk-topped-up",
		Name:      "topped-up",
		Status:    config.ChannelStatusAutoDisabled,
		BaseURL:   &baseURL,
		Models:    "gpt-4o-mini",
		TestModel: "gpt-4o-mini",
	}
	if err := model.DB.Create(channel).Error; err != nil {
		t.Fatal(err)
	}

	title := fmt.Sprintf("(#%d) has been re-enabled", channel.Id)
	before := testNotifier.countContaining(title)
	if err := testAllChannels(false, true); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for channelStatus(t, channel.Id) != config.ChannelStatusEnabled {
		if time.Now().After(deadline) {
			t.Fatal("scheduled recovery check did not re-enable the channel")
		}
		time.Sleep(20 * time.Millisecond)
	}
	for testNotifier.countContaining(title) == before {
		if time.Now().After(deadline) {
			t.Fatal("scheduled recovery check sent no re-enabled notice")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// The unusable-channel rules hold whether or not automatic disabling is on; only
// ShouldDisableChannel depends on the switch.
func TestIsChannelUnusableErrorIgnoresAutoDisableSwitch(t *testing.T) {
	oldAuto := config.AutomaticDisableChannelEnabled
	config.AutomaticDisableChannelEnabled = false
	t.Cleanup(func() { config.AutomaticDisableChannelEnabled = oldAuto })
	oldKeywords := common.DisableChannelKeywordsInstance.GetKeywords()
	common.DisableChannelKeywordsInstance.Load(common.GetDefaultDisableChannelKeywords())
	t.Cleanup(func() { common.DisableChannelKeywordsInstance.Load(oldKeywords) })

	credit := &types.OpenAIErrorWithStatusCode{StatusCode: http.StatusBadRequest}
	credit.OpenAIError.Message = "Your credit balance is too low to access the API."
	if !IsChannelUnusableError(config.ChannelTypeOpenAI, credit) {
		t.Fatal("credit balance error not classified as unusable")
	}
	if ShouldDisableChannel(config.ChannelTypeOpenAI, credit) {
		t.Fatal("ShouldDisableChannel = true with automatic disabling off")
	}

	plain := &types.OpenAIErrorWithStatusCode{StatusCode: http.StatusBadRequest}
	plain.OpenAIError.Message = "messages: field required"
	plain.OpenAIError.Type = "invalid_request_error"
	if IsChannelUnusableError(config.ChannelTypeOpenAI, plain) {
		t.Fatal("a malformed request was classified as an unusable channel")
	}
}
