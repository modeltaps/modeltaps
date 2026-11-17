package relay

import (
	"github.com/modeltaps/modeltaps/common/audioduration"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
)

// probeAudioSeconds measures the uploaded audio and stores the length to bill (the larger
// of what its header declares and the floor its size allows, see audioduration.Length)
// for pre-consuming minutes prices. It returns 0 when the format is not known.
func probeAudioSeconds(c *gin.Context, request *types.AudioRequest) float64 {
	length, ok := audioduration.FromFileHeader(request.File)
	if !ok {
		return 0
	}
	seconds := length.Billable()
	c.Set(config.GinAudioSecondsEstimateKey, seconds)
	return seconds
}

// setAudioSeconds fills usage.AudioSeconds, which minutes prices bill on.
//
// The uploader writes the file header and can shape the transcript, so only lengths the
// upstream states itself are taken as they are:
//  1. the length the provider set (from its own usage report);
//  2. usage.seconds or duration in a JSON response;
//  3. otherwise the larger of the probed upload (already bounded below by its size) and
//     the end of the last segment or subtitle cue in the response: cue times stop where
//     speech stops and text can be made to look like a cue, so they only ever add;
//  4. 60 s when nothing tells.
func setAudioSeconds(c *gin.Context, usage *types.Usage, response *types.AudioResponseWrapper) {
	if usage == nil || usage.AudioSeconds > 0 {
		return
	}
	probed := c.GetFloat64(config.GinAudioSecondsEstimateKey)
	if response != nil {
		contentType := response.Headers["Content-Type"]
		if seconds, ok := audioduration.Declared(response.Body, contentType); ok {
			usage.AudioSeconds = seconds
			return
		}
		if seconds, ok := audioduration.FromResponse(response.Body, contentType); ok {
			usage.AudioSeconds = max(seconds, probed)
			return
		}
	}
	if probed > 0 {
		usage.AudioSeconds = probed
		return
	}
	logger.LogWarn(c.Request.Context(), "audio length unknown, billing the default length")
	usage.AudioSeconds = config.DefaultAudioSeconds
}
