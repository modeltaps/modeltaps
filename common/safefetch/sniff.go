package safefetch

import (
	"bytes"
	"strings"
)

const (
	MimePNG  = "image/png"
	MimeICO  = "image/x-icon"
	MimeJPEG = "image/jpeg"
	MimeGIF  = "image/gif"
	MimeWEBP = "image/webp"
	MimeSVG  = "image/svg+xml"
)

// SniffImage 按文件头判断图片类型，完全忽略服务端声明的 Content-Type；无法识别返回空串。
func SniffImage(b []byte) string {
	switch {
	case bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")):
		return MimePNG
	case bytes.HasPrefix(b, []byte{0, 0, 1, 0}) || bytes.HasPrefix(b, []byte{0, 0, 2, 0}):
		return MimeICO
	case bytes.HasPrefix(b, []byte{0xFF, 0xD8, 0xFF}):
		return MimeJPEG
	case bytes.HasPrefix(b, []byte("GIF87a")) || bytes.HasPrefix(b, []byte("GIF89a")):
		return MimeGIF
	case len(b) >= 12 && bytes.Equal(b[:4], []byte("RIFF")) && bytes.Equal(b[8:12], []byte("WEBP")):
		return MimeWEBP
	case looksLikeSVG(b):
		return MimeSVG
	}
	return ""
}

func looksLikeSVG(b []byte) bool {
	head := b
	if len(head) > 1024 {
		head = head[:1024]
	}
	s := strings.ToLower(strings.TrimLeft(string(bytes.TrimPrefix(head, []byte("\xef\xbb\xbf"))), " \t\r\n"))
	if !strings.HasPrefix(s, "<") {
		return false
	}
	return strings.Contains(s, "<svg") && !strings.Contains(s, "<html")
}
