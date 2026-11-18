package admintools

import (
	"context"
	"io/fs"
	"sort"
	"strings"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	opsdocs "github.com/modeltaps/modeltaps/docs/ops"
)

// docSection is one heading of an operator docs page and the text under it.
type docSection struct {
	Page    string `json:"page"`
	Heading string `json:"heading"`
	Text    string `json:"text"`
}

const maxSectionText = 2000

var docSections = loadDocSections()

// loadDocSections splits every embedded page at its ## and ### headings.
func loadDocSections() []docSection {
	var out []docSection
	_ = fs.WalkDir(opsdocs.English, "en", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		b, err := opsdocs.English.ReadFile(path)
		if err != nil {
			return nil
		}
		page := strings.TrimPrefix(path, "en/")
		heading := page
		var text strings.Builder
		flush := func() {
			if t := strings.TrimSpace(text.String()); t != "" {
				out = append(out, docSection{Page: page, Heading: heading, Text: t})
			}
			text.Reset()
		}
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") || strings.HasPrefix(line, "# ") {
				flush()
				heading = strings.TrimSpace(strings.TrimLeft(line, "#"))
				continue
			}
			text.WriteString(line)
			text.WriteString("\n")
		}
		flush()
		return nil
	})
	return out
}

type docsParams struct {
	Query string `json:"query" description:"Words to look for in the operator docs (all must match): concepts like pricing, supply order, fee rates, pause rules, deployment." required:"true"`
	Limit int    `json:"limit" description:"At most this many sections (default 5, at most 20)." required:"false"`
}

type docsTool struct{}

func (docsTool) GetTool() *protocol.Tool {
	t, _ := protocol.NewTool("settings_docs",
		"Search the Modeltaps operator docs (English): guides such as prices, costs and supply order, supply routing, "+
			"deployment, and the generated settings reference. Returns the matching sections with their page "+
			"(relative to docs/ops/en/) and heading. Use it for how settings work together; settings_describe explains one setting.",
		docsParams{})
	return t
}

func (docsTool) HandleRequest(_ context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	var p docsParams
	if err := protocol.VerifyAndUnmarshal(req.RawArguments, &p); err != nil {
		return nil, err
	}
	limit := p.Limit
	if limit <= 0 {
		limit = 5
	}
	limit = min(limit, 20)
	words := strings.Fields(strings.ToLower(p.Query))
	type hit struct {
		s     docSection
		score int
	}
	var hits []hit
	for _, s := range docSections {
		head, body := strings.ToLower(s.Heading), strings.ToLower(s.Text)
		score := 0
		ok := len(words) > 0
		for _, w := range words {
			switch {
			case strings.Contains(head, w):
				score += 10
			case strings.Contains(body, w):
				score += strings.Count(body, w)
			default:
				ok = false
			}
		}
		if ok {
			// Guides before the generated reference, which repeats settings_describe.
			if !strings.HasPrefix(s.Page, "settings/") {
				score += 5
			}
			hits = append(hits, hit{s, score})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	out := []docSection{}
	for _, h := range hits[:min(len(hits), limit)] {
		s := h.s
		if r := []rune(s.Text); len(r) > maxSectionText {
			s.Text = string(r[:maxSectionText]) + "…"
		}
		out = append(out, s)
	}
	return jsonResult(map[string]any{"total": len(hits), "sections": out})
}
