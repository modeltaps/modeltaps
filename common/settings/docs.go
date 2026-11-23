package settings

import (
	"fmt"
	"sort"
	"strings"
)

// The settings reference of the operator docs (docs/ops/<language>/settings/), generated from the
// catalog and its translations: one page per settings page group, one for the object fields and an
// index. `go generate ./common/settings` writes it; a test keeps it in step.

// DocsLanguage is one language of the reference.
type DocsLanguage struct {
	Code string // en_US, zh_CN, zh_HK, ja_JP
	Dir  string // the language's directory under docs/ops ("" for Simplified Chinese, the default)
	// Catalog holds the language's settingsCatalog translations (nil for English), GroupTitles and
	// CardTitles the console's names of the settings page groups and cards, ObjectTitles the names
	// of the object kinds (scopes).
	Catalog      map[string]any
	GroupTitles  map[string]string
	CardTitles   map[string]string
	ObjectTitles map[string]string
}

var docsWords = map[string]map[string]string{
	"en_US": {"title": "Settings reference", "intro": "Every setting, what it does, where to change it and examples. Generated from the settings catalog (common/settings): do not edit by hand.",
		"objects": "Object fields", "objectsIntro": "Fields set on channels, models, supplies, the allocation policy, user groups, routing policies, organizations and API keys.",
		"key": "Key", "type": "Type", "unit": "Unit", "range": "Range", "default": "Default", "where": "Where", "values": "Values", "examples": "Examples", "related": "See also", "docs": "Guide", "advanced": "rarely changed", "platform": "Platform settings", "secret": "secret: never shown after saving",
		"defaultEmpty": "empty", "colon": ": "},
	"zh_CN": {"title": "设置参考", "intro": "每项设置的作用、在哪里改，以及示例。由设置目录（common/settings）生成，请勿手改。",
		"objects": "对象字段", "objectsIntro": "在渠道、模型、线路、分配策略、用户分组、路由策略、组织和 API 密钥上设置的字段。",
		"key": "设置名", "type": "类型", "unit": "单位", "range": "范围", "default": "默认值", "where": "位置", "values": "取值", "examples": "示例", "related": "相关设置", "docs": "指南", "advanced": "很少需要改", "platform": "平台设置", "secret": "密钥：保存后不再显示",
		"defaultEmpty": "空", "colon": "："},
	"zh_HK": {"title": "設定參考", "intro": "每項設定的作用、在哪裡改，以及示例。由設定目錄（common/settings）生成，請勿手改。",
		"objects": "物件欄位", "objectsIntro": "在渠道、模型、線路、分配策略、用戶分組、路由策略、組織和 API 密鑰上設定的欄位。",
		"key": "設定名", "type": "類型", "unit": "單位", "range": "範圍", "default": "預設值", "where": "位置", "values": "取值", "examples": "示例", "related": "相關設定", "docs": "指南", "advanced": "很少需要改", "platform": "平台設定", "secret": "密鑰：儲存後不再顯示",
		"defaultEmpty": "空", "colon": "："},
	"ja_JP": {"title": "設定リファレンス", "intro": "各設定の働き、変更する場所、例。設定カタログ（common/settings）から生成されます。手で編集しないでください。",
		"objects": "オブジェクトの項目", "objectsIntro": "チャネル、モデル、供給、割り当てポリシー、ユーザーグループ、ルーティングポリシー、組織、API キーに設定する項目。",
		"key": "キー", "type": "型", "unit": "単位", "range": "範囲", "default": "既定値", "where": "場所", "values": "値", "examples": "例", "related": "関連する設定", "docs": "ガイド", "advanced": "ほとんど変更しない", "platform": "プラットフォーム設定", "secret": "シークレット：保存後は表示されません",
		"defaultEmpty": "空", "colon": "："},
}

// docsUnits translates the units the catalog uses.
var docsUnits = map[string]map[string]string{
	"zh_CN": {"s": "秒", "seconds": "秒", "minutes": "分钟", "days": "天", "requests / minute": "次请求 / 分钟", "USD / 1M tokens": "USD / 百万 token"},
	"zh_HK": {"s": "秒", "seconds": "秒", "minutes": "分鐘", "days": "天", "requests / minute": "次請求 / 分鐘", "USD / 1M tokens": "USD / 百萬 token"},
	"ja_JP": {"s": "秒", "seconds": "秒", "minutes": "分", "days": "日", "requests / minute": "リクエスト / 分", "USD / 1M tokens": "USD / 100 万トークン"},
}

func (l DocsLanguage) unit(u string) string {
	if t := docsUnits[l.Code][u]; t != "" {
		return t
	}
	return u
}

// lookup reads a translation of key's part (label, description, examples.0, enum.x.label ...).
func (l DocsLanguage) lookup(key, part string) string {
	if l.Catalog == nil {
		return ""
	}
	var node any = l.Catalog
	for _, seg := range append(strings.Split(key, "."), strings.Split(part, ".")...) {
		switch m := node.(type) {
		case map[string]any:
			node = m[seg]
		case []any:
			var i int
			if _, err := fmt.Sscanf(seg, "%d", &i); err != nil || i < 0 || i >= len(m) {
				return ""
			}
			node = m[i]
		default:
			return ""
		}
	}
	s, _ := node.(string)
	return s
}

func (l DocsLanguage) text(key, part, english string) string {
	if t := l.lookup(key, part); t != "" {
		return t
	}
	return english
}

func (l DocsLanguage) groupTitle(group string) string {
	if t := l.GroupTitles[group]; t != "" {
		return t
	}
	return GroupTitles[group]
}

// pageObjects are the objects of the console pages a site setting can live on (Setting.Page).
var pageObjects = map[string]Scope{"routing-policies": ScopeRoutingPolicy}

// pageTitle names a console page outside the settings page, as the console calls its objects.
func (l DocsLanguage) pageTitle(page string) string {
	if t := l.ObjectTitles[string(pageObjects[page])]; t != "" {
		return t
	}
	return GroupTitles[page]
}

func (l DocsLanguage) cardTitle(card string) string {
	if t := l.CardTitles[card]; t != "" {
		return t
	}
	return card
}

// docsPage is the page an entry is on, without the directory: "gateway.md", "objects.md".
func docsPage(s Setting) string {
	if s.Scope == ScopeSite {
		return s.Group + ".md"
	}
	return "objects.md"
}

// anchor is the HTML id of an entry's heading.
func anchor(key string) string { return strings.ReplaceAll(key, ".", "-") }

// RenderDocs renders the reference of one language: file paths relative to docs/ops mapped to their
// content. defaults are the site settings' defaults (config.GlobalOption.Defaults()).
func RenderDocs(l DocsLanguage, defaults map[string]string) map[string]string {
	w := docsWords[l.Code]
	if w == nil {
		w = docsWords["en_US"]
	}
	base := "settings/"
	if l.Dir != "" {
		base = l.Dir + "/settings/"
	}
	byPage := map[string][]Setting{}
	var pages []string
	for _, s := range All() {
		p := docsPage(s)
		if _, ok := byPage[p]; !ok {
			pages = append(pages, p)
		}
		byPage[p] = append(byPage[p], s)
	}
	files := map[string]string{}
	var index strings.Builder
	fmt.Fprintf(&index, "%s# %s\n\n%s\n\n", frontMatter(w["title"]), w["title"], w["intro"])
	for _, p := range pages {
		list := byPage[p]
		var title string
		if p == "objects.md" {
			title = w["objects"]
		} else {
			title = w["platform"] + " · " + l.groupTitle(list[0].Group)
		}
		fmt.Fprintf(&index, "- [%s](./%s) (%d)\n", title, p, len(list))
		files[base+p] = renderPage(l, w, title, p, list, defaults)
	}
	files[base+"index.md"] = index.String()
	return files
}

func frontMatter(title string) string {
	return fmt.Sprintf("---\ntitle: %q\nlayout: doc\noutline: deep\n---\n\n<!-- Generated by `go generate ./common/settings`. Do not edit. -->\n\n", title)
}

func renderPage(l DocsLanguage, w map[string]string, title, page string, list []Setting, defaults map[string]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s# %s\n\n", frontMatter(title), title)
	if page == "objects.md" {
		fmt.Fprintf(&b, "%s\n\n", w["objectsIntro"])
	}
	section := ""
	for _, s := range list {
		head := l.cardTitle(s.Card)
		if s.Scope != ScopeSite {
			head = l.ObjectTitles[string(s.Scope)]
			if head == "" {
				head = string(s.Scope)
			}
		}
		if head != section {
			fmt.Fprintf(&b, "## %s\n\n", head)
			section = head
		}
		renderEntry(&b, l, w, s, defaults)
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func renderEntry(b *strings.Builder, l DocsLanguage, w map[string]string, s Setting, defaults map[string]string) {
	label := l.text(s.Key, "label", s.Label)
	fmt.Fprintf(b, "<a id=\"%s\"></a>\n\n### %s\n\n", anchor(s.Key), label)
	facts := []string{fmt.Sprintf("%s `%s`", w["key"], s.Key), fmt.Sprintf("%s %s", w["type"], s.Type)}
	if s.Unit != "" {
		facts = append(facts, fmt.Sprintf("%s %s", w["unit"], l.unit(s.Unit)))
	}
	if s.Min != nil || s.Max != nil {
		r := ""
		if s.Min != nil {
			r += trimFloat(*s.Min)
		}
		r += "–"
		if s.Max != nil {
			r += trimFloat(*s.Max)
		}
		facts = append(facts, fmt.Sprintf("%s %s", w["range"], r))
	}
	switch {
	case s.Type == TypeSecret:
		facts = append(facts, w["secret"])
	case s.Scope == ScopeSite:
		if def, ok := defaults[s.Key]; ok {
			shown := def
			if shown == "" {
				shown = w["defaultEmpty"]
			} else if len(shown) > 80 || strings.Contains(shown, "\n") {
				shown = ""
			}
			if shown != "" {
				facts = append(facts, fmt.Sprintf("%s `%s`", w["default"], strings.ReplaceAll(shown, "`", "'")))
			}
		}
	case s.Default != "":
		facts = append(facts, fmt.Sprintf("%s %s", w["default"], s.Default))
	}
	if s.Advanced {
		facts = append(facts, w["advanced"])
	}
	fmt.Fprintf(b, "%s\n\n", strings.Join(facts, " · "))
	where := l.groupTitle(s.Group) + " › " + l.cardTitle(s.Card)
	switch {
	case s.Scope == ScopeSite && s.Page != "":
		where = l.pageTitle(s.Page) + " › " + l.cardTitle(s.Card)
	case s.Scope == ScopeSite:
		where = w["platform"] + " › " + where
	}
	fmt.Fprintf(b, "%s%s%s\n\n", w["where"], w["colon"], where)
	fmt.Fprintf(b, "%s\n\n", l.text(s.Key, "description", s.Description))
	if len(s.Enum) > 0 {
		fmt.Fprintf(b, "%s%s\n\n", w["values"], strings.TrimSpace(w["colon"]))
		for _, e := range s.Enum {
			line := fmt.Sprintf("- `%s` %s", e.Value, l.text(s.Key, "enum."+e.Value+".label", e.Label))
			if e.Description != "" {
				line += w["colon"] + l.text(s.Key, "enum."+e.Value+".description", e.Description)
			}
			fmt.Fprintln(b, line)
		}
		fmt.Fprintln(b)
	}
	if len(s.Examples) > 0 {
		fmt.Fprintf(b, "%s%s\n\n", w["examples"], strings.TrimSpace(w["colon"]))
		for i, ex := range s.Examples {
			fmt.Fprintf(b, "- `%s` %s\n", strings.ReplaceAll(ex.Value, "`", "'"), l.text(s.Key, fmt.Sprintf("examples.%d", i), ex.Effect))
		}
		fmt.Fprintln(b)
	}
	if len(s.Related) > 0 {
		var links []string
		for _, r := range s.Related {
			rel, ok := Get(r)
			if !ok {
				continue
			}
			links = append(links, fmt.Sprintf("[%s](./%s#%s)", l.text(r, "label", rel.Label), docsPage(rel), anchor(r)))
		}
		fmt.Fprintf(b, "%s%s%s\n\n", w["related"], w["colon"], strings.Join(links, " · "))
	}
	if s.Docs != "" {
		fmt.Fprintf(b, "%s%s[%s](../%s)\n\n", w["docs"], w["colon"], s.Docs, s.Docs)
	}
}

func trimFloat(v float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.4f", v), "0"), ".")
}

// DocsFiles renders every language; the keys are paths relative to docs/ops.
func DocsFiles(langs []DocsLanguage, defaults map[string]string) map[string]string {
	out := map[string]string{}
	for _, l := range langs {
		for p, c := range RenderDocs(l, defaults) {
			out[p] = c
		}
	}
	return out
}

// SortedPaths lists the keys of a file map in order.
func SortedPaths(files map[string]string) []string {
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}
