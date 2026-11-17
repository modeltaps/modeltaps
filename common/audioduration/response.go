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
	if !isJSON(body, contentType) {
		return fromSubtitles(strings.TrimSpace(string(body)))
	}
	declared, lastSegment, ok := fromJSON(body)
	if !ok {
		return 0, false
	}
	if declared > 0 {
		return declared, true
	}
	return lastSegment, lastSegment > 0
}

// Declared reads the audio length a JSON response states outright: usage.seconds (type
// "duration"), then duration. The upstream writes these fields itself and bills on them.
// Segment and cue end times are not declarations: they mark where speech ends, and a
// plain-text body can carry text that merely looks like a cue.
func Declared(body []byte, contentType string) (seconds float64, ok bool) {
	if !isJSON(body, contentType) {
		return 0, false
	}
	declared, _, _ := fromJSON(body)
	return declared, declared > 0
}

func isJSON(body []byte, contentType string) bool {
	return strings.HasPrefix(strings.TrimSpace(string(body)), "{") || strings.Contains(contentType, "json")
}

func fromJSON(body []byte) (declared, lastSegment float64, ok bool) {
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
		return 0, 0, false
	}
	switch {
	case resp.Usage != nil && resp.Usage.Seconds > 0 && (resp.Usage.Type == "" || resp.Usage.Type == "duration"):
		declared = resp.Usage.Seconds
	case resp.Duration > 0:
		declared = resp.Duration
	}
	for _, s := range resp.Segments {
		if s.End > lastSegment {
			lastSegment = s.End
		}
	}
	return declared, lastSegment, true
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
