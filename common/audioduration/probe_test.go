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

// mp3Frames is an ID3 tag followed by MPEG-1 Layer III frames at 128 kbps / 44.1 kHz
// (417 bytes, 1152 samples each).
func mp3Frames(seconds float64) []byte {
	out := []byte("ID3\x04\x00\x00\x00\x00\x00\x0A")
	out = append(out, make([]byte, 10)...)
	frames := int(math.Round(seconds * 44100 / 1152))
	for i := 0; i < frames; i++ {
		frame := make([]byte, 417)
		copy(frame, []byte{0xFF, 0xFB, 0x90, 0x00})
		out = append(out, frame...)
	}
	return out
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
		"mp3":  {mp3Frames(20), 20},
	}
	for name, tc := range cases {
		got, ok := Probe(bytes.NewReader(tc.data), int64(len(tc.data)))
		if !ok || !near(got.Seconds, tc.want) || !near(got.Billable(), tc.want) {
			t.Errorf("%s: Probe = %+v, %v; want %v", name, got, ok, tc.want)
		}
	}
}

// forgedWAV is 12.5 s of 16 kHz mono 16-bit PCM whose byte_rate claims 0xFFFFFFFF.
func forgedWAV() []byte {
	b := wavBytes(12.5, 32000)
	binary.LittleEndian.PutUint32(b[28:32], 0xFFFFFFFF)
	return b
}

// forgedXingMP3 is 60 s of MP3 frames whose first frame carries a Xing header claiming
// a single frame.
func forgedXingMP3() []byte {
	b := mp3Frames(60)
	first := b[20:]
	copy(first[36:], "Xing")
	binary.BigEndian.PutUint32(first[40:44], 1) // frame count present
	binary.BigEndian.PutUint32(first[44:48], 1) // one frame
	return b
}

// TestProbeForgedHeaders checks that header fields the uploader can forge do not bring
// the billed length below what the payload really holds.
func TestProbeForgedHeaders(t *testing.T) {
	// FLAC: 16 kHz mono 16-bit, STREAMINFO claims 1 sample, 10 s of raw samples follow.
	flac := flacBytes(16000, 1)
	flac[8+13] |= 0xF0 // bits per sample field 15 -> 16 bits
	flac = append(flac, make([]byte, 16000*2*10)...)

	// Ogg Opus: last granule claims 0.01 s, 512 kB of packets follow (8 s at 510 kb/s).
	opus := opusBytes(0.01, 312)
	for i := 0; i < 2048; i++ {
		opus = append(opus, oggPage(uint64(480+312), make([]byte, 250))...)
	}
	opus = append(opus, oggPage(uint64(480+312), make([]byte, 100))...)

	// MP4 AAC stereo 48 kHz: mvhd claims 1 ms, the mdat holds 720 kB (10 s at 576 kb/s).
	mp4 := mp4WithAudio(1000, 1, "mp4a", 2, 48000, 720_000)

	// MP3: two valid frames, then 1 MB of bytes that are not frames (no 0xFF).
	mp3 := mp3Frames(0.05)
	junk := bytes.Repeat([]byte{0x55}, 1<<20)
	mp3 = append(mp3, junk...)

	// WAV: the data chunk claims 2 bytes and 12.5 s of samples follow it.
	shortData := wavBytes(12.5, 32000)
	binary.LittleEndian.PutUint32(shortData[40:44], 2)

	cases := map[string]struct {
		data    []byte
		atLeast float64
	}{
		"wav byte_rate":  {forgedWAV(), 12.5},
		"wav data size":  {shortData, 12.5},
		"mp3 xing count": {forgedXingMP3(), 60},
		"mp3 junk":       {mp3, float64(len(mp3)-20) * 8 / mp3MaxBitrate},
		"flac samples":   {flac, 10 * 16.0 / 17},
		"opus granule":   {opus, float64(len(opus)-200) * 8 / opusMaxBitrate},
		"mp4 mvhd":       {mp4, 10},
	}
	for name, tc := range cases {
		got, ok := Probe(bytes.NewReader(tc.data), int64(len(tc.data)))
		if !ok || got.Billable() < tc.atLeast-0.05 {
			t.Errorf("%s: Probe = %+v (billable %v), %v; want at least %v", name, got, got.Billable(), ok, tc.atLeast)
		}
	}
}

// TestProbeFloorNeverExceedsHonestLength checks that the size floor stays below the real
// length of honest files, so billing the larger of the two never over-charges them.
func TestProbeFloorNeverExceedsHonestLength(t *testing.T) {
	cases := map[string]struct {
		data []byte
		want float64
	}{
		"wav":           {wavBytes(12.5, 32000), 12.5},
		"mp3":           {mp3Frames(30), 30},
		"mp4 aac 128k":  {mp4WithAudio(1000, 10_000, "mp4a", 2, 44100, 160_000), 10},
		"mp4 alac 24b":  {mp4WithAudio(1000, 10_000, "alac", 2, 96000, 96000*2*3*10), 10},
		"opus with art": {opusWithCoverArt(), 4},
	}
	for name, tc := range cases {
		got, ok := Probe(bytes.NewReader(tc.data), int64(len(tc.data)))
		if !ok || got.Floor > tc.want+0.05 || !near(got.Billable(), tc.want) {
			t.Errorf("%s: Probe = %+v, %v; want %v with the floor below it", name, got, ok, tc.want)
		}
	}
}

// mp4WithAudio is an MP4 with one sound track and an mdat of mdat bytes. An "mp4a" track
// carries an AAC-LC decoder configuration for the given rate and channels; the sample
// entry fields are left inflated, as a forger would set them.
func mp4WithAudio(timescale, duration uint32, codec string, channels int, sampleRate float64, mdat int) []byte {
	box := func(typ string, body ...[]byte) []byte {
		joined := bytes.Join(body, nil)
		b := binary.BigEndian.AppendUint32(nil, uint32(8+len(joined)))
		return append(append(b, typ...), joined...)
	}
	mvhd := make([]byte, 100)
	binary.BigEndian.PutUint32(mvhd[12:16], timescale)
	binary.BigEndian.PutUint32(mvhd[16:20], duration)
	hdlr := append(make([]byte, 8), "soun"...)
	hdlr = append(hdlr, make([]byte, 13)...)
	entry := make([]byte, 28)
	binary.BigEndian.PutUint16(entry[16:18], 0xFFFF) // channel count
	binary.BigEndian.PutUint16(entry[18:20], 16)
	binary.BigEndian.PutUint16(entry[24:26], 0xFFFF) // sample rate
	if codec == "mp4a" {
		freq := 0
		for i, rate := range aacSampleRates {
			if rate == sampleRate {
				freq = i
			}
		}
		asc := binary.BigEndian.AppendUint16(nil, uint16(2<<11|freq<<7|channels<<3))
		config := append([]byte{0x40, 0x15}, make([]byte, 11)...)
		config = append(config, 0x05, byte(len(asc)))
		config = append(config, asc...)
		es := append([]byte{0, 1, 0, 0x04, byte(len(config))}, config...)
		esds := append(make([]byte, 4), 0x03, byte(len(es)))
		entry = append(entry, box("esds", append(esds, es...))...)
	}
	stsd := append(binary.BigEndian.AppendUint32(make([]byte, 4), 1), box(codec, entry)...)
	trak := box("trak", box("mdia", box("hdlr", hdlr), box("minf", box("stbl", box("stsd", stsd)))))
	out := box("ftyp", []byte("M4A \x00\x00\x00\x00"))
	out = append(out, box("moov", box("mvhd", mvhd), trak)...)
	return append(out, box("mdat", make([]byte, mdat))...)
}

// opusWithCoverArt is 4 s of 32 kb/s Opus behind 600 kB of tag pages (a cover picture).
func opusWithCoverArt() []byte {
	head := append([]byte("OpusHead"), 1, 2)
	head = binary.LittleEndian.AppendUint16(head, 312)
	head = append(head, make([]byte, 9)...)
	out := oggPage(0, head)
	for i := 0; i < 2400; i++ {
		out = append(out, oggPage(0, make([]byte, 250))...)
	}
	for i := 1; i <= 64; i++ { // 64 pages of 250 bytes: 16 kB, 4 s at 32 kb/s
		out = append(out, oggPage(uint64(i*3000+312), make([]byte, 250))...)
	}
	return out
}

func TestProbeUnknown(t *testing.T) {
	for name, data := range map[string][]byte{
		"empty": nil,
		"text":  []byte("this is not audio at all, just some words"),
		"webm":  {0x1A, 0x45, 0xDF, 0xA3, 0, 0, 0, 0, 0, 0, 0, 0},
	} {
		if got, ok := Probe(bytes.NewReader(data), int64(len(data))); ok {
			t.Errorf("%s: Probe = %+v, want not ok", name, got)
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
		length, ok := Probe(bytes.NewReader(data), int64(len(data)))
		got := length.Billable()
		if !ok || math.Abs(got-37.5) > 0.2 || length.Floor > 37.5+0.2 {
			t.Errorf("%s: Probe = %v, %v; want 37.5", filepath.Base(path), got, ok)
		} else {
			t.Logf("%s: %.3f s (floor %.3f s)", filepath.Base(path), got, length.Floor)
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

func TestDeclared(t *testing.T) {
	cases := map[string]struct {
		body, contentType string
		want              float64
		ok                bool
	}{
		"usage seconds": {`{"text":"x","usage":{"type":"duration","seconds":42}}`, "application/json", 42, true},
		"duration":      {`{"text":"x","duration":7.5,"usage":{"type":"tokens","input_tokens":10}}`, "application/json", 7.5, true},
		"segments only": {`{"text":"x","segments":[{"end":3.2},{"end":9.75}]}`, "application/json", 0, false},
		"srt":           {"1\n00:00:00,000 --> 00:00:04,000\nhi\n", "text/plain", 0, false},
		"text":          {"hello --> 00:00:01,000", "text/plain", 0, false},
	}
	for name, tc := range cases {
		got, ok := Declared([]byte(tc.body), tc.contentType)
		if ok != tc.ok || !near(got, tc.want) {
			t.Errorf("%s: Declared = %v, %v; want %v, %v", name, got, ok, tc.want, tc.ok)
		}
	}
}
