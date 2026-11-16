package audioduration

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// FromResponse reads the audio length a transcription response reports. JSON bodies use
// usage.seconds (type "duration"), then duration, then the end of the last segment; SRT
// and VTT bodies use the end of the last cue. ok is false when the body says nothing.
func FromResponse(body []byte, contentType string) (seconds float64, ok bool) {
	trimmed := strings.TrimSpace(string(body))
	if strings.HasPrefix(trimmed, "{") || strings.Contains(contentType, "json") {
		return fromJSON(body)
	}
	return fromSubtitles(trimmed)
}

func fromJSON(body []byte) (float64, bool) {
	var resp struct {
		Duration float64 `json:"duration"`
		Usage    *struct {
			Type    string  `json:"type"`
			Seconds float64 `json:"seconds"`
		} `json:"usage"`
		Segments []struct {
			End float64 `json:"end"`
		} `json:"segments"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, false
	}
	if resp.Usage != nil && resp.Usage.Seconds > 0 && (resp.Usage.Type == "" || resp.Usage.Type == "duration") {
		return resp.Usage.Seconds, true
	}
	if resp.Duration > 0 {
		return resp.Duration, true
	}
	last := 0.0
	for _, s := range resp.Segments {
		if s.End > last {
			last = s.End
		}
	}
	return last, last > 0
}

// cueEnd matches the end time of an SRT ("00:01:02,500") or VTT ("01:02.500") cue.
var cueEnd = regexp.MustCompile(`-->\s*(?:(\d+):)?(\d{1,2}):(\d{2})[.,](\d{1,3})`)

func fromSubtitles(text string) (float64, bool) {
	last := 0.0
	for _, m := range cueEnd.FindAllStringSubmatch(text, -1) {
		h, _ := strconv.Atoi(m[1])
		min, _ := strconv.Atoi(m[2])
		sec, _ := strconv.Atoi(m[3])
		frac, _ := strconv.ParseFloat("0."+m[4], 64)
		if t := float64(h*3600+min*60+sec) + frac; t > last {
			last = t
		}
	}
	return last, last > 0
}
