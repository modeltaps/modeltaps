package relay

import (
	"github.com/modeltaps/modeltaps/common"
	providersBase "github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/types"
	"net/http"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

type relaySpeech struct {
	relayBase
	request types.SpeechAudioRequest
}

func NewRelaySpeech(c *gin.Context) *relaySpeech {
	relay := &relaySpeech{}
	relay.c = c
	return relay
}

func (r *relaySpeech) setRequest() error {
	if err := common.UnmarshalBodyReusable(r.c, &r.request); err != nil {
		return err
	}

	r.setOriginalModel(r.request.Model)

	return nil
}

func (r *relaySpeech) getPromptTokens() (int, error) {
	return speechInputChars(r.request.Input), nil
}

// speechInputChars counts TTS input in characters (runes), not UTF-8 bytes.
func speechInputChars(input string) int {
	return utf8.RuneCountInString(input)
}

func (r *relaySpeech) send() (err *types.OpenAIErrorWithStatusCode, done bool) {
	provider, ok := r.provider.(providersBase.SpeechInterface)
	if !ok {
		err = common.StringErrorWrapperLocal("channel not implemented", "channel_error", http.StatusServiceUnavailable)
		done = true
		return
	}

	r.request.Model = r.modelName

	response, err := provider.CreateSpeech(&r.request)
	if err != nil {
		return
	}
	err = responseMultipart(r.c, response)

	if err != nil {
		done = true
	}

	return
}
