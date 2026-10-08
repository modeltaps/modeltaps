package safefetch

import (
	"strings"
	"testing"
)

func TestSanitizeSVGStripsMaliciousContent(t *testing.T) {
	in := `<?xml version="1.0"?>
<!-- comment -->
<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:evil="urn:x" viewBox="0 0 24 24" onload="alert(1)">
  <script>alert(1)</script>
  <style>@import url(https://evil.example/x.css);</style>
  <foreignObject><iframe src="javascript:alert(1)"></iframe></foreignObject>
  <a href="javascript:alert(1)"><path d="M0 0"/></a>
  <set attributeName="href" to="javascript:alert(1)"/>
  <image href="https://evil.example/track.png"/>
  <use xlink:href="https://evil.example/x.svg#a"/>
  <use xlink:href="#ok"/>
  <rect evil:x="1" fill="url(https://evil.example/p)" style="fill:url(#g)" onclick="x()" width="1"/>
  <path d="M1 1" style="background:url('javascript:alert(1)')"/>
  <circle r="1" style="fill:\75rl(https://evil.example/esc)"/>
</svg>`
	out, err := SanitizeSVG([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, bad := range []string{"script", "alert", "style>", "@import", "foreignObject", "iframe", "javascript",
		"<a", "<set", "<image", "evil", "onload", "onclick", "<!--", "<?xml"} {
		if strings.Contains(s, bad) {
			t.Errorf("sanitized svg still contains %q: %s", bad, s)
		}
	}
	for _, good := range []string{`xmlns="http://www.w3.org/2000/svg"`, `viewBox="0 0 24 24"`, `xlink:href="#ok"`,
		`style="fill:url(#g)"`, `width="1"`, `<path d="M1 1">`} {
		if !strings.Contains(s, good) {
			t.Errorf("sanitized svg lost %q: %s", good, s)
		}
	}
	if SniffImage(out) != MimeSVG {
		t.Errorf("sanitized output should still sniff as svg")
	}
}

func TestSanitizeSVGRejectsInvalid(t *testing.T) {
	for _, in := range []string{
		"",
		"<html><body>hi</body></html>",
		`<svg xmlns="http://www.w3.org/2000/svg"><path></svg>`,
		`<!DOCTYPE svg [<!ENTITY x "boom">]><svg xmlns="http://www.w3.org/2000/svg">&x;</svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg"/><svg xmlns="http://www.w3.org/2000/svg"/>`,
	} {
		if _, err := SanitizeSVG([]byte(in)); err == nil {
			t.Errorf("want error for %q", in)
		}
	}
}

func TestSniffImage(t *testing.T) {
	cases := map[string]string{
		"\x89PNG\r\n\x1a\n....":                 MimePNG,
		"\x00\x00\x01\x00....":                  MimeICO,
		"\xff\xd8\xff\xe0":                      MimeJPEG,
		"GIF89a..":                              MimeGIF,
		"RIFF\x00\x00\x00\x00WEBPVP8 ":          MimeWEBP,
		"\xef\xbb\xbf  <svg viewBox='0 0 1 1'>": MimeSVG,
		"<!doctype html><html><svg></svg>":      "",
		"plain text":                            "",
	}
	for in, want := range cases {
		if got := SniffImage([]byte(in)); got != want {
			t.Errorf("SniffImage(%q) = %q, want %q", in, got, want)
		}
	}
}
