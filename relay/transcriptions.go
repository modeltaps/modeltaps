package relay

import (
	"github.com/modeltaps/modeltaps/common"
	providersBase "github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/types"
	"net/http"

	"github.com/gin-gonic/gin"
)

type relayTranscriptions struct {
	relayBase
	request types.AudioRequest
}

func NewRelayTranscriptions(c *gin.Context) *relayTranscriptions {
	relay := &relayTranscriptions{}
	relay.c = c
	return relay
}

func (r *relayTranscriptions) setRequest() error {
	if err := common.UnmarshalBodyReusable(r.c, &r.request); err != nil {
		return err
	}

	r.request.FormValues = multipartValues(r.c)
	probeAudioSeconds(r.c, &r.request)
	r.setOriginalModel(r.request.Model)

	return nil
}

// multipartValues copies the parsed non-file form fields so providers that rebuild the
// upstream form can forward them unchanged.
func multipartValues(c *gin.Context) map[string][]string {
	if c.Request.MultipartForm == nil {
		return nil
	}
	values := make(map[string][]string, len(c.Request.MultipartForm.Value))
	for key, v := range c.Request.MultipartForm.Value {
		values[key] = append([]string(nil), v...)
	}
	return values
}

func (r *relayTranscriptions) getPromptTokens() (int, error) {
	return 0, nil
}

func (r *relayTranscriptions) send() (err *types.OpenAIErrorWithStatusCode, done bool) {
	provider, ok := r.provider.(providersBase.TranscriptionsInterface)
	if !ok {
		err = common.StringErrorWrapperLocal("channel not implemented", "channel_error", http.StatusServiceUnavailable)
		done = true
		return
	}

	r.request.Model = r.modelName

	response, err := provider.CreateTranscriptions(&r.request)
	if err != nil {
		return
	}
	setAudioSeconds(r.c, r.provider.GetUsage(), response)
	err = responseCustom(r.c, response)

	if err != nil {
		done = true
	}

	return
}
