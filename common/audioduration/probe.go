// Package audioduration works out how long an audio clip is, for billing speech-to-text
// models by the minute. It reads the duration from container headers (WAV, FLAC, Ogg,
// MP3, MP4/M4A) and from transcription responses (JSON usage/duration, SRT/VTT cues).
//
// The uploader controls every header field, so a header alone can be forged to claim a
// clip a thousand times shorter than the audio it carries. Probe therefore reports two
// numbers: the length the header declares, checked against the fields a decoder actually
// uses where the format allows it, and a floor worked out from the size of the audio
// payload and the highest bitrate the declared stream can carry. An honest file is never
// shorter than its floor, so billing the larger of the two never over-charges it, while a
// forged header can at most bring the bill down to the floor.
package audioduration

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"mime/multipart"
)

// Length is what the container of an upload says about its duration.
type Length struct {
	// Seconds is the length the header declares, verified where the format allows (WAV
	// lengths come from the sample format, MP3 lengths from counting the frames).
	Seconds float64
	// Floor is the shortest length the audio payload can have: its size over the highest
	// bitrate the declared stream can carry. 0 when the format gives no bound.
	Floor float64
}

// Billable is the length to bill: the larger of the declared length and the floor.
func (l Length) Billable() float64 {
	if l.Floor > l.Seconds {
		return l.Floor
	}
	return l.Seconds
}

// FromFileHeader probes an uploaded multipart file. ok is false when the format is
// unknown or the header cannot be parsed.
func FromFileHeader(fh *multipart.FileHeader) (length Length, ok bool) {
	if fh == nil {
		return Length{}, false
	}
	f, err := fh.Open()
	if err != nil {
		return Length{}, false
	}
	defer f.Close()
	return Probe(f, fh.Size)
}

// Probe reads the duration of r, which holds size bytes, from its container.
func Probe(r io.ReaderAt, size int64) (length Length, ok bool) {
	head := make([]byte, 12)
	if n, _ := r.ReadAt(head, 0); n < 4 {
		return Length{}, false
	}
	switch {
	case bytes.HasPrefix(head, []byte("RIFF")) && bytes.Equal(head[8:12], []byte("WAVE")):
		length, ok = probeWAV(r, size)
	case bytes.HasPrefix(head, []byte("fLaC")):
		length, ok = probeFLAC(r, size)
	case bytes.HasPrefix(head, []byte("OggS")):
		length, ok = probeOgg(r, size)
	case bytes.Equal(head[4:8], []byte("ftyp")):
		length, ok = probeMP4(r, size)
	default:
		length, ok = probeMP3(r, size)
	}
	if !ok || length.Billable() <= 0 {
		return Length{}, false
	}
	return length, true
}

func readAt(r io.ReaderAt, off int64, n int) []byte {
	if off < 0 || n <= 0 {
		return nil
	}
	buf := make([]byte, n)
	m, _ := r.ReadAt(buf, off)
	return buf[:m]
}

// floorSeconds is the length of payload bytes played at the highest bitrate (bits per
// second) the stream can carry.
func floorSeconds(payload int64, maxBitrate float64) float64 {
	if payload <= 0 || maxBitrate <= 0 {
		return 0
	}
	return float64(payload) * 8 / maxBitrate
}

// WAV format tags whose samples are stored as plain fixed-size values.
var wavPCMFormats = map[uint16]bool{
	0x0001: true, // PCM
	0x0003: true, // IEEE float
	0x0006: true, // A-law
	0x0007: true, // mu-law
	0xFFFE: true, // WAVE_FORMAT_EXTENSIBLE
}

func probeWAV(r io.ReaderAt, size int64) (Length, bool) {
	var rate float64 // bytes per second of the data chunk
	var data int64
	seenData := false
	off := int64(12)
	for off+8 <= size {
		h := readAt(r, off, 8)
		if len(h) < 8 {
			break
		}
		if !printableID(h[:4]) {
			// Garbage after the chunks a decoder knows: count it as audio rather than
			// trusting a data size that would end the payload early.
			if seenData {
				data += size - off
			}
			break
		}
		id := string(h[:4])
		chunkSize := int64(binary.LittleEndian.Uint32(h[4:8]))
		switch id {
		case "fmt ":
			fmtChunk := readAt(r, off+8, 16)
			if len(fmtChunk) < 16 {
				return Length{}, false
			}
			rate = wavByteRate(fmtChunk)
		case "data":
			if rate <= 0 {
				return Length{}, false
			}
			// Streamed WAVs leave the data size as 0 or 0xFFFFFFFF; use the bytes present.
			if chunkSize == 0 || chunkSize == 0xFFFFFFFF || off+8+chunkSize > size {
				chunkSize = size - off - 8
			}
			data += chunkSize
			seenData = true
		}
		off += 8 + chunkSize + chunkSize%2
	}
	if !seenData || rate <= 0 {
		return Length{}, false
	}
	seconds := float64(data) / rate
	return Length{Seconds: seconds, Floor: seconds}, true
}

// wavByteRate works out the bytes per second of a WAV data chunk. The byte_rate field is
// advisory and decoders of plain samples ignore it, so for those formats the rate comes
// from sample rate x channels x sample width, the values a decoder plays the data at.
// Compressed formats keep byte_rate, capped at the rate of 32-bit samples.
func wavByteRate(fmtChunk []byte) float64 {
	format := binary.LittleEndian.Uint16(fmtChunk[0:2])
	channels := float64(binary.LittleEndian.Uint16(fmtChunk[2:4]))
	sampleRate := float64(binary.LittleEndian.Uint32(fmtChunk[4:8]))
	byteRate := float64(binary.LittleEndian.Uint32(fmtChunk[8:12]))
	bits := binary.LittleEndian.Uint16(fmtChunk[14:16])
	if wavPCMFormats[format] {
		return sampleRate * channels * float64((bits+7)/8)
	}
	if limit := sampleRate * channels * 4; byteRate <= 0 || byteRate > limit {
		return limit
	}
	return byteRate
}

func printableID(id []byte) bool {
	for _, b := range id {
		if b < 0x20 || b > 0x7E {
			return false
		}
	}
	return true
}

func probeFLAC(r io.ReaderAt, size int64) (Length, bool) {
	// The first metadata block must be STREAMINFO; its body starts at byte 8.
	info := readAt(r, 8, 18)
	if len(info) < 18 {
		return Length{}, false
	}
	sampleRate := uint32(info[10])<<12 | uint32(info[11])<<4 | uint32(info[12])>>4
	channels := uint32(info[12]>>1&0x07) + 1
	bitsPerSample := uint32(info[12]&0x01)<<4 | uint32(info[13]>>4) + 1
	totalSamples := uint64(info[13]&0x0F)<<32 | uint64(binary.BigEndian.Uint32(info[14:18]))
	if sampleRate == 0 {
		return Length{}, false
	}
	var length Length
	if totalSamples > 0 {
		length.Seconds = float64(totalSamples) / float64(sampleRate)
	}

	// Frames start after the last metadata block (pictures and tags can be large). FLAC
	// never stores more than the raw samples plus frame headers; one extra bit per sample
	// covers the headers.
	off := int64(4)
	for off+4 <= size {
		h := readAt(r, off, 4)
		if len(h) < 4 {
			break
		}
		off += 4 + (int64(h[1])<<16 | int64(h[2])<<8 | int64(h[3]))
		if h[0]&0x80 != 0 {
			break
		}
	}
	length.Floor = floorSeconds(size-off, float64(sampleRate*channels*(bitsPerSample+1)))
	return length, true
}

// opusMaxBitrate is the highest bitrate of one Opus stream (RFC 6716 allows 510 kb/s).
const opusMaxBitrate = 512_000

func probeOgg(r io.ReaderAt, size int64) (Length, bool) {
	// The first packet tells the codec and sample rate; it starts after the page header
	// (27 bytes) and the segment table.
	first := readAt(r, 0, 27+255+64)
	if len(first) < 28 {
		return Length{}, false
	}
	segments := int(first[26])
	body := 27 + segments
	if len(first) < body+20 {
		return Length{}, false
	}
	packet := first[body:]
	var sampleRate uint32
	var preSkip uint16
	var maxBitrate float64
	switch {
	case bytes.HasPrefix(packet, []byte("OpusHead")):
		sampleRate = 48000 // Opus granule positions are always in 48 kHz samples
		preSkip = binary.LittleEndian.Uint16(packet[10:12])
		// Mapping family 0 is one mono or stereo stream; other families state the number
		// of streams the packets are split into.
		maxBitrate = opusMaxBitrate
		if packet[18] != 0 {
			maxBitrate = opusMaxBitrate * float64(max(packet[19], 1))
		}
	case bytes.HasPrefix(packet, []byte("\x01vorbis")):
		sampleRate = binary.LittleEndian.Uint32(packet[12:16])
		// Vorbis stays well below 16-bit samples at any quality setting.
		maxBitrate = float64(sampleRate) * float64(max(packet[11], 1)) * 16
	default:
		return Length{}, false
	}
	if sampleRate == 0 {
		return Length{}, false
	}

	var length Length
	length.Floor = floorSeconds(size-oggAudioStart(r, size), maxBitrate)

	// The last page's granule position is the total sample count.
	const tail = 64 * 1024
	start := size - tail
	if start < 0 {
		start = 0
	}
	buf := readAt(r, start, int(size-start))
	if idx := bytes.LastIndex(buf, []byte("OggS")); idx >= 0 && idx+14 <= len(buf) {
		granule := binary.LittleEndian.Uint64(buf[idx+6 : idx+14])
		if granule != 0 && granule != ^uint64(0) && granule > uint64(preSkip) {
			length.Seconds = float64(granule-uint64(preSkip)) / float64(sampleRate)
		}
	}
	return length, true
}

// oggAudioStart walks the header pages (granule position 0: codec setup, tags, cover art)
// and returns the offset of the first audio page.
func oggAudioStart(r io.ReaderAt, size int64) int64 {
	off := int64(0)
	for off+27 <= size {
		h := readAt(r, off, 27)
		if len(h) < 27 || !bytes.HasPrefix(h, []byte("OggS")) {
			return off
		}
		if binary.LittleEndian.Uint64(h[6:14]) != 0 {
			return off
		}
		table := readAt(r, off+27, int(h[26]))
		if len(table) < int(h[26]) {
			return off
		}
		next := off + 27 + int64(len(table))
		for _, s := range table {
			next += int64(s)
		}
		off = next
	}
	return off
}

func probeMP4(r io.ReaderAt, size int64) (Length, bool) {
	moov, moovSize, ok := findBox(r, 0, size, "moov")
	if !ok {
		return Length{}, false
	}
	var length Length
	if mvhd, _, ok := findBox(r, moov, moov+moovSize, "mvhd"); ok {
		length.Seconds = mvhdSeconds(readAt(r, mvhd, 32))
	}
	length.Floor = floorSeconds(mdatBytes(r, size), mp4MaxBitrate(r, moov, moovSize))
	return length, true
}

func mvhdSeconds(b []byte) float64 {
	if len(b) < 20 {
		return 0
	}
	var timescale uint32
	var duration uint64
	if b[0] == 1 {
		if len(b) < 32 {
			return 0
		}
		timescale = binary.BigEndian.Uint32(b[20:24])
		duration = binary.BigEndian.Uint64(b[24:32])
	} else {
		timescale = binary.BigEndian.Uint32(b[12:16])
		duration = uint64(binary.BigEndian.Uint32(b[16:20]))
	}
	if timescale == 0 || duration == 0 {
		return 0
	}
	return float64(duration) / float64(timescale)
}

// mdatBytes adds up the media data boxes at the top level of the file.
func mdatBytes(r io.ReaderAt, size int64) int64 {
	var total int64
	off := int64(0)
	for off < size {
		body, bodySize, ok := findBox(r, off, size, "mdat")
		if !ok {
			break
		}
		total += bodySize
		off = body + bodySize
	}
	return total
}

// mp4MaxBitrate returns the highest bitrate the first sound track can carry, taken from
// the decoder configuration (the fields a decoder plays the stream with; the channel and
// rate fields of the sample entry are advisory and could be inflated freely): AAC stores
// at most 6144 bits per 1024 samples per channel, MP3 at most 320 kb/s. Other codecs give
// no bound (0).
func mp4MaxBitrate(r io.ReaderAt, moov, moovSize int64) float64 {
	end := moov + moovSize
	off := moov
	for off < end {
		trak, trakSize, ok := findBox(r, off, end, "trak")
		if !ok {
			return 0
		}
		off = trak + trakSize
		mdia, mdiaSize, ok := findBox(r, trak, trak+trakSize, "mdia")
		if !ok {
			continue
		}
		hdlr, _, ok := findBox(r, mdia, mdia+mdiaSize, "hdlr")
		if !ok || string(readAt(r, hdlr+8, 4)) != "soun" {
			continue
		}
		minf, minfSize, ok := findBox(r, mdia, mdia+mdiaSize, "minf")
		if !ok {
			return 0
		}
		stbl, stblSize, ok := findBox(r, minf, minf+minfSize, "stbl")
		if !ok {
			return 0
		}
		stsd, stsdSize, ok := findBox(r, stbl, stbl+stblSize, "stsd")
		if !ok {
			return 0
		}
		// stsd: version/flags, entry count, then the first sample entry box, whose
		// AudioSampleEntry fields take 28 bytes before its child boxes.
		entry, entrySize, ok := findBox(r, stsd+8, stsd+stsdSize, "mp4a")
		if !ok {
			return 0
		}
		esds, esdsSize, ok := findBox(r, entry+28, entry+entrySize, "esds")
		if !ok || esdsSize < 4 || esdsSize > 1024 {
			return 0
		}
		return esdsMaxBitrate(readAt(r, esds+4, int(esdsSize-4)))
	}
	return 0
}

var aacSampleRates = [13]float64{96000, 88200, 64000, 48000, 44100, 32000, 24000, 22050, 16000, 12000, 11025, 8000, 7350}

// esdsMaxBitrate walks the MPEG-4 ES descriptor to the decoder configuration.
func esdsMaxBitrate(b []byte) float64 {
	// descriptor reads a tag and its length (1 to 4 bytes of 7 bits) and returns the body.
	descriptor := func(b []byte, tag byte) []byte {
		if len(b) < 2 || b[0] != tag {
			return nil
		}
		n, i := 0, 1
		for ; i < len(b) && i <= 4; i++ {
			n = n<<7 | int(b[i]&0x7F)
			if b[i]&0x80 == 0 {
				break
			}
		}
		i++
		if i > len(b) || n > len(b)-i {
			return nil
		}
		return b[i : i+n]
	}
	es := descriptor(b, 0x03)
	if len(es) < 3 {
		return 0
	}
	flags := es[2]
	es = es[3:]
	if flags&0x80 != 0 { // stream dependence
		es = es[min(2, len(es)):]
	}
	if flags&0x40 != 0 && len(es) > 0 { // URL
		es = es[min(1+int(es[0]), len(es)):]
	}
	if flags&0x20 != 0 { // OCR stream
		es = es[min(2, len(es)):]
	}
	config := descriptor(es, 0x04)
	if len(config) < 13 {
		return 0
	}
	switch config[0] { // objectTypeIndication
	case 0x69, 0x6B: // MPEG-2 / MPEG-1 audio (MP3)
		return mp3MaxBitrate
	case 0x40, 0x66, 0x67, 0x68: // MPEG-4 AAC, MPEG-2 AAC profiles
	default:
		return 0
	}
	asc := descriptor(config[13:], 0x05)
	if len(asc) < 2 {
		return 0
	}
	bits := uint32(asc[0])<<24 | uint32(asc[1])<<16
	if len(asc) > 2 {
		bits |= uint32(asc[2]) << 8
	}
	if len(asc) > 3 {
		bits |= uint32(asc[3])
	}
	pos := 5
	if bits>>27 == 31 { // escaped object type
		pos += 6
	}
	freqIndex := bits << pos >> 28
	pos += 4
	if freqIndex >= uint32(len(aacSampleRates)) {
		return 0 // explicit or reserved rate
	}
	channelConfig := bits << pos >> 28
	channels := float64(channelConfig)
	if channelConfig == 0 || channelConfig >= 7 {
		channels = 8 // program config element or 7.1 layouts
	}
	return aacSampleRates[freqIndex] * channels * 6
}

// findBox scans the boxes in [start, end) and returns the offset and size of the body of
// the first box of the given type.
func findBox(r io.ReaderAt, start, end int64, boxType string) (int64, int64, bool) {
	off := start
	for off+8 <= end {
		h := readAt(r, off, 16)
		if len(h) < 8 {
			return 0, 0, false
		}
		boxSize := int64(binary.BigEndian.Uint32(h[:4]))
		header := int64(8)
		switch boxSize {
		case 1:
			if len(h) < 16 {
				return 0, 0, false
			}
			boxSize = int64(binary.BigEndian.Uint64(h[8:16]))
			header = 16
		case 0:
			boxSize = end - off
		}
		if boxSize < header {
			return 0, 0, false
		}
		if boxSize > end-off { // truncated upload: count what is there
			boxSize = end - off
		}
		if string(h[4:8]) == boxType {
			return off + header, boxSize - header, true
		}
		off += boxSize
	}
	return 0, 0, false
}

var (
	mp3Bitrates = map[int][16]int{ // kbps, index 0 and 15 invalid
		1: {0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0}, // MPEG-1 Layer III
		2: {0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0},     // MPEG-2/2.5 Layer III
	}
	mp3SampleRates = map[int][3]int{
		1: {44100, 48000, 32000},
		2: {22050, 24000, 16000},
		3: {11025, 12000, 8000}, // MPEG-2.5
	}
)

// mp3MaxBitrate is the highest Layer III bitrate (MPEG-1, 320 kb/s).
const mp3MaxBitrate = 320_000

// mp3Frame is a parsed Layer III frame header.
type mp3Frame struct {
	length  int     // bytes, header included
	seconds float64 // audio the frame decodes to
}

func parseMP3Frame(hdr []byte) (mp3Frame, bool) {
	if len(hdr) < 4 || hdr[0] != 0xFF || hdr[1]&0xE0 != 0xE0 {
		return mp3Frame{}, false
	}
	versionBits := (hdr[1] >> 3) & 0x03
	layerBits := (hdr[1] >> 1) & 0x03
	if versionBits == 1 || layerBits != 1 { // reserved version, or not Layer III
		return mp3Frame{}, false
	}
	version := map[byte]int{3: 1, 2: 2, 0: 3}[versionBits]
	bitrateIdx := int(hdr[2] >> 4)
	rateIdx := int((hdr[2] >> 2) & 0x03)
	if bitrateIdx == 0 || bitrateIdx == 15 || rateIdx == 3 {
		return mp3Frame{}, false
	}
	table, samplesPerFrame, coefficient := 1, 1152, 144
	if version != 1 {
		table, samplesPerFrame, coefficient = 2, 576, 72
	}
	bitrate := mp3Bitrates[table][bitrateIdx] * 1000
	sampleRate := mp3SampleRates[version][rateIdx]
	padding := int(hdr[2]>>1) & 0x01
	return mp3Frame{
		length:  coefficient*bitrate/sampleRate + padding,
		seconds: float64(samplesPerFrame) / float64(sampleRate),
	}, true
}

// probeMP3 counts the frames a decoder would play. Xing/VBRI frame counts and the bitrate
// of the first frame are header claims; walking the frames (and resyncing over junk, as
// decoders do) measures the audio that is really there.
func probeMP3(r io.ReaderAt, size int64) (Length, bool) {
	off := int64(0)
	if h := readAt(r, 0, 10); len(h) == 10 && bytes.HasPrefix(h, []byte("ID3")) {
		tagSize := int64(h[6]&0x7F)<<21 | int64(h[7]&0x7F)<<14 | int64(h[8]&0x7F)<<7 | int64(h[9]&0x7F)
		off = 10 + tagSize
		if h[5]&0x10 != 0 {
			off += 10 // footer
		}
	}

	// Find the first frame sync within a short window after the tag. A frame counts as
	// the start of the stream only when the next frame follows it (or the file ends),
	// so stray 0xFF bytes in other formats are not taken for MP3.
	window := readAt(r, off, 64*1024)
	start := int64(-1)
	for i := 0; i+4 <= len(window); i++ {
		frame, ok := parseMP3Frame(window[i : i+4])
		if !ok {
			continue
		}
		next := off + int64(i) + int64(frame.length)
		if next >= size {
			start = off + int64(i)
			break
		}
		if _, ok := parseMP3Frame(readAt(r, next, 4)); ok {
			start = off + int64(i)
			break
		}
	}
	if start < 0 {
		return Length{}, false
	}
	return Length{
		Seconds: walkMP3(r, start, size),
		Floor:   floorSeconds(size-start, mp3MaxBitrate),
	}, true
}

func walkMP3(r io.ReaderAt, start, size int64) float64 {
	br := bufio.NewReaderSize(io.NewSectionReader(r, start, size-start), 64*1024)
	seconds := 0.0
	for {
		hdr, err := br.Peek(4)
		if len(hdr) < 4 {
			return seconds
		}
		if frame, ok := parseMP3Frame(hdr); ok && frame.length > 4 {
			seconds += frame.seconds
			if _, err := br.Discard(frame.length); err != nil {
				return seconds
			}
			continue
		}
		if err != nil {
			return seconds
		}
		// Not a frame: skip to the next 0xFF.
		buf, _ := br.Peek(br.Buffered())
		skip := len(buf)
		if i := bytes.IndexByte(buf[1:], 0xFF); i >= 0 {
			skip = i + 1
		}
		if _, err := br.Discard(skip); err != nil {
			return seconds
		}
	}
}
