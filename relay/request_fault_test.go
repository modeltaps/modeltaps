package relay

import (
	"net/http"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/types"
)

func faultErr(status int, message string) *types.OpenAIErrorWithStatusCode {
	return &types.OpenAIErrorWithStatusCode{StatusCode: status, OpenAIError: types.OpenAIError{Message: message}}
}

// A plain 400 is about the request and is not held against the channel; a 400 that blames the
// channel (empty balance, rejected key, a thought signature it cannot check), a missing model
// and every other status are the channel's.
func TestIsRequestFault(t *testing.T) {
	geminiKey := faultErr(http.StatusBadRequest, "API key not valid. Please pass a valid API key.")
	geminiKey.OpenAIError.Param = "INVALID_ARGUMENT"
	quota := faultErr(http.StatusBadRequest, "You exceeded your current quota")
	quota.OpenAIError.Type = "insufficient_quota"

	cases := []struct {
		name        string
		err         *types.OpenAIErrorWithStatusCode
		channelType int
		want        bool
	}{
		{"malformed body", faultErr(http.StatusBadRequest, "messages: at least one message is required"), config.ChannelTypeAnthropic, true},
		{"context too long", faultErr(http.StatusBadRequest, "prompt is too long: 250000 tokens > 200000 maximum"), config.ChannelTypeAnthropic, true},
		{"unsupported parameter", faultErr(http.StatusBadRequest, "Unsupported parameter: 'logit_bias'"), config.ChannelTypeOpenAI, true},
		{"anthropic balance", faultErr(http.StatusBadRequest, "Your credit balance is too low to access the API"), config.ChannelTypeAnthropic, false},
		{"bedrock not allowed", faultErr(http.StatusBadRequest, "Operation not allowed"), config.ChannelTypeBedrock, false},
		{"gemini key", geminiKey, config.ChannelTypeGemini, false},
		{"gemini thought signature", faultErr(http.StatusBadRequest, "Thought signature is not valid"), config.ChannelTypeGemini, false},
		{"quota as a 400", quota, config.ChannelTypeOpenAI, false},
		{"model missing", faultErr(http.StatusBadRequest, "The model `gpt-x` does not exist"), config.ChannelTypeOpenAI, false},
		{"rate limit", faultErr(http.StatusTooManyRequests, "slow down"), config.ChannelTypeOpenAI, false},
		{"upstream failure", faultErr(http.StatusInternalServerError, "boom"), config.ChannelTypeOpenAI, false},
		{"too large", faultErr(http.StatusRequestEntityTooLarge, "request too large"), config.ChannelTypeOpenAI, false},
		{"gateway's own refusal", apiErr(http.StatusBadRequest, true), config.ChannelTypeOpenAI, false},
		{"none", nil, config.ChannelTypeOpenAI, false},
	}
	for _, tc := range cases {
		if got := isRequestFault(tc.channelType, tc.err); got != tc.want {
			t.Errorf("%s: isRequestFault = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// What is not retried as a 400 is exactly what counts as the request's fault, so a channel is
// never both spared the blame and skipped for the next channel.
func TestRequestFaultMatchesRetry(t *testing.T) {
	for _, tc := range []struct {
		message     string
		channelType int
	}{
		{"messages: at least one message is required", config.ChannelTypeAnthropic},
		{"Your credit balance is too low to access the API", config.ChannelTypeAnthropic},
		{"Operation not allowed", config.ChannelTypeBedrock},
		{"Thought signature is not valid", config.ChannelTypeGemini},
	} {
		e := faultErr(http.StatusBadRequest, tc.message)
		retried := shouldRetry(newTestContext(), e, tc.channelType)
		if fault := isRequestFault(tc.channelType, e); fault == retried {
			t.Errorf("%q: request fault = %v and retried = %v", tc.message, fault, retried)
		}
	}
}
