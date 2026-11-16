package relay

import (
	"github.com/modeltaps/modeltaps/common/audioduration"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
)

// probeAudioSeconds reads the length of the uploaded audio from its container header and
// stores it for pre-consuming minutes prices. It returns 0 when the format is not known.
func probeAudioSeconds(c *gin.Context, request *types.AudioRequest) float64 {
	seconds, ok := audioduration.FromFileHeader(request.File)
	if !ok {
		return 0
	}
	c.Set(config.GinAudioSecondsEstimateKey, seconds)
	return seconds
}

// setAudioSeconds fills usage.AudioSeconds, which minutes prices bill on. The length comes
// from the provider if it set one, then the upstream response (usage.seconds, duration,
// segment or cue end times), then the probed file, and 60 s when nothing tells.
func setAudioSeconds(c *gin.Context, usage *types.Usage, response *types.AudioResponseWrapper) {
	if usage == nil || usage.AudioSeconds > 0 {
		return
	}
	if response != nil {
		if seconds, ok := audioduration.FromResponse(response.Body, response.Headers["Content-Type"]); ok {
			usage.AudioSeconds = seconds
			return
		}
	}
	if seconds := c.GetFloat64(config.GinAudioSecondsEstimateKey); seconds > 0 {
		usage.AudioSeconds = seconds
		return
	}
	logger.LogWarn(c.Request.Context(), "audio length unknown, billing the default length")
	usage.AudioSeconds = config.DefaultAudioSeconds
}
