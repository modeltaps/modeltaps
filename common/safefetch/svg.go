package safefetch

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
)

var ErrBadSVG = errors.New("safefetch: invalid or unsupported svg")

const (
	nsSVG   = "http://www.w3.org/2000/svg"
	nsXLink = "http://www.w3.org/1999/xlink"
)

// 白名单：只保留纯绘制元素；不在表内的元素连同子树一并丢弃（script、style、foreignObject、
// 动画元素、image/use 外链等都不在表内）。
var allowedSVGElements = map[string]bool{
	"svg": true, "g": true, "defs": true, "title": true, "desc": true, "symbol": true, "use": true,
	"path": true, "rect": true, "circle": true, "ellipse": true, "line": true, "polyline": true,
	"polygon": true, "text": true, "tspan": true, "clipPath": true, "mask": true, "pattern": true,
	"linearGradient": true, "radialGradient": true, "stop": true, "filter": true,
	"feGaussianBlur": true, "feOffset": true, "feBlend": true, "feColorMatrix": true,
	"feComposite": true, "feFlood": true, "feMerge": true, "feMergeNode": true, "feMorphology": true,
}

// SanitizeSVG 以白名单重写 SVG：去掉非白名单元素、事件属性、非本文档内片段引用，
// 注释/处理指令/DOCTYPE 一律丢弃；根元素必须是 <svg>。
func SanitizeSVG(in []byte) ([]byte, error) {
	dec := xml.NewDecoder(bytes.NewReader(in))
	dec.Strict = true
	var out bytes.Buffer
	depth, skip := 0, 0
	sawRoot := false
	for {
		tok, err := dec.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, ErrBadSVG
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if skip > 0 {
				skip++
				continue
			}
			if depth == 0 {
				if sawRoot || t.Name.Space != "" || t.Name.Local != "svg" {
					return nil, ErrBadSVG
				}
				sawRoot = true
			}
			if t.Name.Space != "" || !allowedSVGElements[t.Name.Local] {
				skip = 1
				continue
			}
			depth++
			out.WriteString("<" + t.Name.Local)
			for _, a := range t.Attr {
				if name, ok := safeSVGAttr(a); ok {
					out.WriteString(" " + name + `="`)
					_ = xml.EscapeText(&out, []byte(a.Value))
					out.WriteString(`"`)
				}
			}
			out.WriteString(">")
		case xml.EndElement:
			if skip > 0 {
				skip--
				continue
			}
			depth--
			out.WriteString("</" + t.Name.Local + ">")
		case xml.CharData:
			if skip == 0 && depth > 0 {
				_ = xml.EscapeText(&out, t)
			}
		}
	}
	if !sawRoot || depth != 0 {
		return nil, ErrBadSVG
	}
	return out.Bytes(), nil
}

func safeSVGAttr(a xml.Attr) (string, bool) {
	local := a.Name.Local
	val := strings.TrimSpace(a.Value)
	lower := strings.ToLower(val)
	switch a.Name.Space {
	case "":
		if local == "xmlns" {
			return local, val == nsSVG
		}
	case "xmlns":
		return "xmlns:" + local, local == "xlink" && val == nsXLink
	case "xlink":
		if local == "href" {
			return "xlink:href", strings.HasPrefix(val, "#")
		}
		return "", false
	default:
		return "", false
	}
	if strings.HasPrefix(strings.ToLower(local), "on") {
		return "", false
	}
	if local == "href" {
		return local, strings.HasPrefix(val, "#")
	}
	if strings.Contains(val, `\`) || strings.Contains(lower, "javascript:") || strings.Contains(lower, "expression(") ||
		strings.Contains(lower, "@import") || hasExternalURL(lower) {
		return "", false
	}
	return local, true
}

// hasExternalURL 报告值中是否含有指向文档外的 url(...) 引用。
func hasExternalURL(v string) bool {
	for {
		i := strings.Index(v, "url(")
		if i < 0 {
			return false
		}
		rest := strings.TrimLeft(v[i+4:], " \t'\"")
		if !strings.HasPrefix(rest, "#") {
			return true
		}
		v = rest
	}
}
