package model

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ModelFamily 是从模型 id 拆出的家族信息，供渠道模型挑选按家族折叠、只默认勾选最新版本。
//
// 规则（以 OpenRouter / Vercel / 各厂商直连的真实 id 为样本）：
//   - 去掉厂商前缀（最后一个 "/" 之前）与变体后缀（":batch"、":free"），变体单独记在 Variant；
//   - 按 "-" 切词，第一个版本号词是 Version，其余非数字词按原顺序拼成 Family：
//     "claude-opus-4-5"、"anthropic/claude-opus-4.5"、"claude-3-5-opus-20240229" 都归 "claude-opus"；
//   - 粘连写法拆开：qwen3.5 → qwen + 3.5，kimi-k2.5 → kimi-k + 2.5，gpt-4o → gpt-o + 4，glm-4.5v → glm-v + 4.5；
//   - 规模词（8b、a22b、8x22b、2.4t）留在 Family 里，不同规模算不同家族；
//   - 版本号之后的纯数字词（0613、20241022、02-15）是快照日期，记在 Snapshot；
//   - preview / exp / beta / alpha 记为 Preview，latest 视为不带快照的别名，均不进 Family；
//   - "~" 开头的 id（OpenRouter 的 "~anthropic/claude-opus-latest"）是跟随最新版的浮动别名，记为 Alias。
type ModelFamily struct {
	Family   string `json:"family"`
	Version  string `json:"version"`
	Snapshot string `json:"snapshot,omitempty"`
	Variant  string `json:"variant,omitempty"`
	Preview  bool   `json:"preview,omitempty"`
	Alias    bool   `json:"alias,omitempty"`
}

var (
	familyNumberPattern = regexp.MustCompile(`^\d+(\.\d+)*$`)
	familySizePattern   = regexp.MustCompile(`^(a?\d+(\.\d+)?[bmt]|\d+x\d+[bmt])$`)
	// 字母前缀 + 版本（qwen3.5、k2.5、r1、o3、v4.1、m3.1），可带字母后缀（v2v）。
	familyGluedPrefixPattern = regexp.MustCompile(`^([a-z]+)(\d+(?:\.\d+)*)([a-z]*)$`)
	// 版本 + 字母后缀（4o、4.5v、5v）。
	familyGluedSuffixPattern = regexp.MustCompile(`^(\d+(?:\.\d+)*)([a-z]+)$`)
)

var familyPreviewWords = map[string]bool{"preview": true, "exp": true, "experimental": true, "beta": true, "alpha": true}

// isSnapshotNumber 判断纯数字词是否是日期快照（0613、2508、20241022），而不是版本号。
func isSnapshotNumber(token string) bool {
	return !strings.Contains(token, ".") && len(token) >= 4
}

// ParseModelFamily 拆解模型 id，见 ModelFamily 的规则说明。
func ParseModelFamily(id string) ModelFamily {
	name := strings.ToLower(strings.TrimSpace(id))
	var info ModelFamily
	if strings.HasPrefix(name, "~") {
		info.Alias = true
		name = name[1:]
	}
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if i := strings.Index(name, ":"); i >= 0 {
		info.Variant = name[i+1:]
		name = name[:i]
	}

	tokens := strings.Split(name, "-")
	familyTokens := make([]string, 0, len(tokens))
	var snapshot []string
	versionFound := false
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		switch {
		case token == "":
			continue
		case familyPreviewWords[token]:
			info.Preview = true
		case token == "latest":
			continue
		case familySizePattern.MatchString(token):
			familyTokens = append(familyTokens, token)
		case familyNumberPattern.MatchString(token):
			if versionFound || isSnapshotNumber(token) {
				snapshot = append(snapshot, token)
				continue
			}
			versionFound = true
			info.Version = token
			// 横线写法的小版本：claude-opus-4-5、mistral-medium-3-5。
			if !strings.Contains(token, ".") && i+1 < len(tokens) && len(tokens[i+1]) <= 2 && familyNumberPattern.MatchString(tokens[i+1]) {
				info.Version += "." + tokens[i+1]
				i++
			}
		case !versionFound && familyGluedPrefixPattern.MatchString(token):
			m := familyGluedPrefixPattern.FindStringSubmatch(token)
			versionFound = true
			info.Version = m[2]
			familyTokens = append(familyTokens, m[1]+m[3])
		case !versionFound && familyGluedSuffixPattern.MatchString(token):
			m := familyGluedSuffixPattern.FindStringSubmatch(token)
			versionFound = true
			info.Version = m[1]
			familyTokens = append(familyTokens, m[2])
		default:
			familyTokens = append(familyTokens, token)
		}
	}
	info.Family = strings.Join(familyTokens, "-")
	info.Snapshot = strings.Join(snapshot, "-")
	return info
}

// CompareModelVersions 比较两个版本号：主版本按整数，其后各段按小数比较（4.20 < 4.3，
// 与厂商实际的发布顺序一致）。空版本最小。返回 -1 / 0 / 1。
func CompareModelVersions(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return -1
	}
	if b == "" {
		return 1
	}
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	ma, _ := strconv.Atoi(pa[0])
	mb, _ := strconv.Atoi(pb[0])
	if ma != mb {
		if ma < mb {
			return -1
		}
		return 1
	}
	for i := 1; i < len(pa) || i < len(pb); i++ {
		var sa, sb string
		if i < len(pa) {
			sa = pa[i]
		}
		if i < len(pb) {
			sb = pb[i]
		}
		for len(sa) < len(sb) {
			sa += "0"
		}
		for len(sb) < len(sa) {
			sb += "0"
		}
		if sa != sb {
			if sa < sb {
				return -1
			}
			return 1
		}
	}
	return 0
}

// ParseCatalogDate 把上游的日期字符串（"2026-04-14"、"2026-04"）转为当天 0 点的 UTC Unix 秒；
// 空串或无法解析返回 0（未知）。
func ParseCatalogDate(value string) int64 {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"2006-01-02", "2006-01"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t.Unix()
		}
	}
	return 0
}
