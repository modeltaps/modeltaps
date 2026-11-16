package audioduration

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func near(got, want float64) bool { return math.Abs(got-want) < 0.05 }

func wavBytes(seconds float64, byteRate uint32) []byte {
	data := make([]byte, int(seconds*float64(byteRate)))
	var b bytes.Buffer
	b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(36+len(data)))
	b.WriteString("WAVEfmt ")
	_ = binary.Write(&b, binary.LittleEndian, uint32(16))
	_ = binary.Write(&b, binary.LittleEndian, []uint16{1, 1}) // PCM, mono
	_ = binary.Write(&b, binary.LittleEndian, []uint32{byteRate / 2, byteRate})
	_ = binary.Write(&b, binary.LittleEndian, []uint16{2, 16})
	b.WriteString("data")
	_ = binary.Write(&b, binary.LittleEndian, uint32(len(data)))
	b.Write(data)
	return b.Bytes()
}

func flacBytes(sampleRate uint32, totalSamples uint64) []byte {
	b := append([]byte("fLaC"), 0x80, 0, 0, 34) // last block, STREAMINFO, length 34
	info := make([]byte, 34)
	info[10] = byte(sampleRate >> 12)
	info[11] = byte(sampleRate >> 4)
	info[12] = byte(sampleRate<<4) | 0x00 // channels/bps bits left zero
	info[13] = byte(totalSamples>>32) & 0x0F
	binary.BigEndian.PutUint32(info[14:18], uint32(totalSamples))
	return append(b, info...)
}

func oggPage(granule uint64, packet []byte) []byte {
	var b bytes.Buffer
	b.WriteString("OggS")
	b.Write([]byte{0, 0})
	_ = binary.Write(&b, binary.LittleEndian, granule)
	b.Write(make([]byte, 12)) // serial, sequence, checksum
	b.WriteByte(1)
	b.WriteByte(byte(len(packet)))
	b.Write(packet)
	return b.Bytes()
}

func opusBytes(seconds float64, preSkip uint16) []byte {
	head := append([]byte("OpusHead"), 1, 1)
	head = binary.LittleEndian.AppendUint16(head, preSkip)
	head = append(head, make([]byte, 9)...)
	out := oggPage(0, head)
	out = append(out, oggPage(0, make([]byte, 100))...)
	return append(out, oggPage(uint64(seconds*48000)+uint64(preSkip), make([]byte, 100))...)
}

func mp4Bytes(timescale, duration uint32) []byte {
	box := func(typ string, body []byte) []byte {
		b := binary.BigEndian.AppendUint32(nil, uint32(8+len(body)))
		return append(append(b, typ...), body...)
	}
	mvhd := make([]byte, 100)
	binary.BigEndian.PutUint32(mvhd[12:16], timescale)
	binary.BigEndian.PutUint32(mvhd[16:20], duration)
	out := box("ftyp", []byte("M4A \x00\x00\x00\x00"))
	out = append(out, box("free", make([]byte, 20))...)
	return append(out, box("moov", box("mvhd", mvhd))...)
}

// cbrMP3 is an ID3 tag followed by MPEG-1 Layer III frames at 128 kbps / 44.1 kHz.
func cbrMP3(seconds float64) []byte {
	out := []byte("ID3\x04\x00\x00\x00\x00\x00\x0A")
	out = append(out, make([]byte, 10)...)
	audio := make([]byte, int(seconds*128000/8))
	copy(audio, []byte{0xFF, 0xFB, 0x90, 0x00})
	return append(out, audio...)
}

func TestProbeSynthetic(t *testing.T) {
	cases := map[string]struct {
		data []byte
		want float64
	}{
		"wav":  {wavBytes(12.5, 32000), 12.5},
		"flac": {flacBytes(16000, 16000*90), 90},
		"opus": {opusBytes(61.25, 312), 61.25},
		"mp4":  {mp4Bytes(1000, 754_321), 754.321},
		"mp3":  {cbrMP3(20), 20},
	}
	for name, tc := range cases {
		got, ok := Probe(bytes.NewReader(tc.data), int64(len(tc.data)))
		if !ok || !near(got, tc.want) {
			t.Errorf("%s: Probe = %v, %v; want %v", name, got, ok, tc.want)
		}
	}
}

func TestProbeUnknown(t *testing.T) {
	for name, data := range map[string][]byte{
		"empty": nil,
		"text":  []byte("this is not audio at all, just some words"),
		"webm":  {0x1A, 0x45, 0xDF, 0xA3, 0, 0, 0, 0, 0, 0, 0, 0},
	} {
		if got, ok := Probe(bytes.NewReader(data), int64(len(data))); ok {
			t.Errorf("%s: Probe = %v, want not ok", name, got)
		}
	}
}

// TestProbeRealFiles checks files encoded by a real encoder. It runs only when
// AUDIO_FIXTURE_DIR points at a directory of 37.5 s clips named out.<ext>.
func TestProbeRealFiles(t *testing.T) {
	dir := os.Getenv("AUDIO_FIXTURE_DIR")
	if dir == "" {
		t.Skip("AUDIO_FIXTURE_DIR not set")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "out.*"))
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		got, ok := Probe(bytes.NewReader(data), int64(len(data)))
		if !ok || math.Abs(got-37.5) > 0.2 {
			t.Errorf("%s: Probe = %v, %v; want 37.5", filepath.Base(path), got, ok)
		} else {
			t.Logf("%s: %.3f s", filepath.Base(path), got)
		}
	}
}

func TestFromResponse(t *testing.T) {
	cases := map[string]struct {
		body, contentType string
		want              float64
	}{
		"usage seconds":   {`{"text":"x","usage":{"type":"duration","seconds":42}}`, "application/json", 42},
		"usage tokens":    {`{"text":"x","duration":7.5,"usage":{"type":"tokens","input_tokens":10}}`, "application/json", 7.5},
		"segments":        {`{"text":"x","segments":[{"end":3.2},{"end":9.75}]}`, "application/json", 9.75},
		"srt":             {"1\n00:00:00,000 --> 00:00:04,000\nhi\n\n2\n00:01:04,000 --> 00:01:10,500\nbye\n", "text/plain", 70.5},
		"vtt short times": {"WEBVTT\n\n00:01.000 --> 00:05.250\nhi\n", "text/vtt", 5.25},
	}
	for name, tc := range cases {
		got, ok := FromResponse([]byte(tc.body), tc.contentType)
		if !ok || !near(got, tc.want) {
			t.Errorf("%s: FromResponse = %v, %v; want %v", name, got, ok, tc.want)
		}
	}
	for _, body := range []string{`{"text":"hello"}`, "plain transcript"} {
		if got, ok := FromResponse([]byte(body), ""); ok {
			t.Errorf("FromResponse(%q) = %v, want not ok", body, got)
		}
	}
}
