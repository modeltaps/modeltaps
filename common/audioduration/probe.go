// Package audioduration works out how long an audio clip is, for billing speech-to-text
// models by the minute. It reads the duration from container headers (WAV, FLAC, Ogg,
// MP3, MP4/M4A) and from transcription responses (JSON usage/duration, SRT/VTT cues).
package audioduration

import (
	"bytes"
	"encoding/binary"
	"io"
	"mime/multipart"
)

// FromFileHeader probes an uploaded multipart file. ok is false when the format is
// unknown or the header cannot be parsed.
func FromFileHeader(fh *multipart.FileHeader) (seconds float64, ok bool) {
	if fh == nil {
		return 0, false
	}
	f, err := fh.Open()
	if err != nil {
		return 0, false
	}
	defer f.Close()
	return Probe(f, fh.Size)
}

// Probe reads the duration from the container header of r, which holds size bytes.
func Probe(r io.ReaderAt, size int64) (seconds float64, ok bool) {
	head := make([]byte, 12)
	if n, _ := r.ReadAt(head, 0); n < 4 {
		return 0, false
	}
	var d float64
	switch {
	case bytes.HasPrefix(head, []byte("RIFF")) && bytes.Equal(head[8:12], []byte("WAVE")):
		d, ok = probeWAV(r, size)
	case bytes.HasPrefix(head, []byte("fLaC")):
		d, ok = probeFLAC(r)
	case bytes.HasPrefix(head, []byte("OggS")):
		d, ok = probeOgg(r, size)
	case bytes.Equal(head[4:8], []byte("ftyp")):
		d, ok = probeMP4(r, size)
	default:
		d, ok = probeMP3(r, size)
	}
	if !ok || d <= 0 {
		return 0, false
	}
	return d, true
}

func readAt(r io.ReaderAt, off int64, n int) []byte {
	if off < 0 || n <= 0 {
		return nil
	}
	buf := make([]byte, n)
	m, _ := r.ReadAt(buf, off)
	return buf[:m]
}

func probeWAV(r io.ReaderAt, size int64) (float64, bool) {
	var byteRate uint32
	off := int64(12)
	for off+8 <= size {
		h := readAt(r, off, 8)
		if len(h) < 8 {
			return 0, false
		}
		id := string(h[:4])
		chunkSize := int64(binary.LittleEndian.Uint32(h[4:8]))
		switch id {
		case "fmt ":
			fmtChunk := readAt(r, off+8, 16)
			if len(fmtChunk) < 16 {
				return 0, false
			}
			byteRate = binary.LittleEndian.Uint32(fmtChunk[8:12])
		case "data":
			if byteRate == 0 {
				return 0, false
			}
			// Streamed WAVs leave the data size as 0 or 0xFFFFFFFF; use the bytes present.
			if chunkSize == 0 || chunkSize == 0xFFFFFFFF || off+8+chunkSize > size {
				chunkSize = size - off - 8
			}
			return float64(chunkSize) / float64(byteRate), true
		}
		off += 8 + chunkSize + chunkSize%2
	}
	return 0, false
}

func probeFLAC(r io.ReaderAt) (float64, bool) {
	// The first metadata block must be STREAMINFO; its body starts at byte 8.
	info := readAt(r, 8, 18)
	if len(info) < 18 {
		return 0, false
	}
	sampleRate := uint32(info[10])<<12 | uint32(info[11])<<4 | uint32(info[12])>>4
	totalSamples := uint64(info[13]&0x0F)<<32 | uint64(binary.BigEndian.Uint32(info[14:18]))
	if sampleRate == 0 || totalSamples == 0 {
		return 0, false
	}
	return float64(totalSamples) / float64(sampleRate), true
}

func probeOgg(r io.ReaderAt, size int64) (float64, bool) {
	// The first packet tells the codec and sample rate; it starts after the page header
	// (27 bytes) and the segment table.
	first := readAt(r, 0, 27+255+64)
	if len(first) < 28 {
		return 0, false
	}
	segments := int(first[26])
	body := 27 + segments
	if len(first) < body+20 {
		return 0, false
	}
	packet := first[body:]
	var sampleRate uint32
	var preSkip uint16
	switch {
	case bytes.HasPrefix(packet, []byte("OpusHead")):
		sampleRate = 48000 // Opus granule positions are always in 48 kHz samples
		preSkip = binary.LittleEndian.Uint16(packet[10:12])
	case bytes.HasPrefix(packet, []byte("\x01vorbis")):
		sampleRate = binary.LittleEndian.Uint32(packet[12:16])
	default:
		return 0, false
	}
	if sampleRate == 0 {
		return 0, false
	}

	// The last page's granule position is the total sample count.
	const tail = 64 * 1024
	start := size - tail
	if start < 0 {
		start = 0
	}
	buf := readAt(r, start, int(size-start))
	idx := bytes.LastIndex(buf, []byte("OggS"))
	if idx < 0 || idx+14 > len(buf) {
		return 0, false
	}
	granule := binary.LittleEndian.Uint64(buf[idx+6 : idx+14])
	if granule == 0 || granule == ^uint64(0) || granule <= uint64(preSkip) {
		return 0, false
	}
	return float64(granule-uint64(preSkip)) / float64(sampleRate), true
}

func probeMP4(r io.ReaderAt, size int64) (float64, bool) {
	moov, moovSize, ok := findBox(r, 0, size, "moov")
	if !ok {
		return 0, false
	}
	mvhd, _, ok := findBox(r, moov, moov+moovSize, "mvhd")
	if !ok {
		return 0, false
	}
	b := readAt(r, mvhd, 32)
	if len(b) < 20 {
		return 0, false
	}
	var timescale uint32
	var duration uint64
	if b[0] == 1 {
		if len(b) < 32 {
			return 0, false
		}
		timescale = binary.BigEndian.Uint32(b[20:24])
		duration = binary.BigEndian.Uint64(b[24:32])
	} else {
		timescale = binary.BigEndian.Uint32(b[12:16])
		duration = uint64(binary.BigEndian.Uint32(b[16:20]))
	}
	if timescale == 0 || duration == 0 {
		return 0, false
	}
	return float64(duration) / float64(timescale), true
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

func probeMP3(r io.ReaderAt, size int64) (float64, bool) {
	off := int64(0)
	if h := readAt(r, 0, 10); len(h) == 10 && bytes.HasPrefix(h, []byte("ID3")) {
		tagSize := int64(h[6]&0x7F)<<21 | int64(h[7]&0x7F)<<14 | int64(h[8]&0x7F)<<7 | int64(h[9]&0x7F)
		off = 10 + tagSize
		if h[5]&0x10 != 0 {
			off += 10 // footer
		}
	}

	// Find the first frame sync within a short window after the tag.
	window := readAt(r, off, 64*1024)
	for i := 0; i+4 <= len(window); i++ {
		if window[i] != 0xFF || window[i+1]&0xE0 != 0xE0 {
			continue
		}
		hdr := window[i : i+4]
		versionBits := (hdr[1] >> 3) & 0x03
		layerBits := (hdr[1] >> 1) & 0x03
		if versionBits == 1 || layerBits != 1 { // reserved version, or not Layer III
			continue
		}
		version := map[byte]int{3: 1, 2: 2, 0: 3}[versionBits]
		bitrateIdx := int(hdr[2] >> 4)
		rateIdx := int((hdr[2] >> 2) & 0x03)
		if bitrateIdx == 0 || bitrateIdx == 15 || rateIdx == 3 {
			continue
		}
		table := 1
		samplesPerFrame := 1152
		if version != 1 {
			table = 2
			samplesPerFrame = 576
		}
		bitrate := mp3Bitrates[table][bitrateIdx] * 1000
		sampleRate := mp3SampleRates[version][rateIdx]

		frameStart := off + int64(i)
		if frames, ok := mp3VBRFrames(window[i:], version, hdr[3]>>6 == 3); ok {
			return float64(frames) * float64(samplesPerFrame) / float64(sampleRate), true
		}
		// Constant bitrate: the audio bytes over the bitrate.
		return float64(size-frameStart) * 8 / float64(bitrate), true
	}
	return 0, false
}

// mp3VBRFrames reads the frame count from a Xing/Info or VBRI header in the first frame.
func mp3VBRFrames(frame []byte, version int, mono bool) (uint32, bool) {
	sideInfo := 32
	switch {
	case version == 1 && mono:
		sideInfo = 17
	case version != 1 && mono:
		sideInfo = 9
	case version != 1:
		sideInfo = 17
	}
	xing := 4 + sideInfo
	if len(frame) >= xing+16 {
		tag := string(frame[xing : xing+4])
		if tag == "Xing" || tag == "Info" {
			flags := binary.BigEndian.Uint32(frame[xing+4 : xing+8])
			if flags&0x1 != 0 {
				return binary.BigEndian.Uint32(frame[xing+8 : xing+12]), true
			}
		}
	}
	const vbri = 4 + 32
	if len(frame) >= vbri+18 && string(frame[vbri:vbri+4]) == "VBRI" {
		return binary.BigEndian.Uint32(frame[vbri+14 : vbri+18]), true
	}
	return 0, false
}
