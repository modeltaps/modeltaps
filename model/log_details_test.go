package model

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/spf13/viper"
)

// TestTruncateLogIOBody 覆盖截断函数的边界与 UTF-8 安全语义:
// <上限不截断且无标志、恰好上限边界、>上限截断且置标志、
// CJK/多字节字符跨边界不产生半个字符(严格 UTF-8、零 U+FFFD)、空输入/nil 安全。
// 上限由 LogIOMaxBodyBytes() 从配置读取,默认 1MB。

// assertNoPartialRune 断言截断结果为严格合法 UTF-8 且不含 U+FFFD 替换符。
func assertNoPartialRune(t *testing.T, got string) {
	t.Helper()
	if !utf8.ValidString(got) {
		t.Fatalf("截断结果不是合法 UTF-8")
	}
	if strings.ContainsRune(got, utf8.RuneError) {
		t.Fatalf("截断结果含 U+FFFD 替换符,出现半个字符")
	}
}

func TestTruncateLogIOBody_BelowLimit(t *testing.T) {
	limit := LogIOMaxBodyBytes()
	body := []byte(strings.Repeat("a", limit-1))
	got, truncated := TruncateLogIOBody(body)
	if truncated {
		t.Fatalf("不足上限不应被截断")
	}
	if len(got) != limit-1 {
		t.Fatalf("长度被改动: got %d want %d", len(got), limit-1)
	}
	if got != string(body) {
		t.Fatalf("内容被改动")
	}
}

func TestTruncateLogIOBody_ExactLimit(t *testing.T) {
	limit := LogIOMaxBodyBytes()
	body := []byte(strings.Repeat("a", limit))
	got, truncated := TruncateLogIOBody(body)
	if truncated {
		t.Fatalf("恰好上限(<=)不应被截断")
	}
	if len(got) != limit {
		t.Fatalf("恰好上限长度被改动: got %d want %d", len(got), limit)
	}
}

func TestTruncateLogIOBody_OverLimitASCII(t *testing.T) {
	limit := LogIOMaxBodyBytes()
	body := []byte(strings.Repeat("a", limit+100))
	got, truncated := TruncateLogIOBody(body)
	if !truncated {
		t.Fatalf("超过上限应被截断并置标志")
	}
	// 纯 ASCII 在边界处无半字符,应精确截到上限。
	if len(got) != limit {
		t.Fatalf("ASCII 截断长度应为上限: got %d want %d", len(got), limit)
	}
	assertNoPartialRune(t, got)
}

func TestTruncateLogIOBody_CJKBoundary(t *testing.T) {
	limit := LogIOMaxBodyBytes()
	// '中' = 3 字节; 默认上限 1MB 时 1048576 % 3 == 1, 边界落在某个字符的第二个字节上,必现半字符。
	cjk := strings.Repeat("中", limit) // 远超上限
	body := []byte(cjk)
	got, truncated := TruncateLogIOBody(body)
	if !truncated {
		t.Fatalf("超过上限的 CJK 应被截断")
	}
	assertNoPartialRune(t, got)
	// 修剪半字符后必然 <= 上限,且为 3 的整数倍(完整的 '中')。
	if len(got) > limit {
		t.Fatalf("截断后不应超过上限: got %d", len(got))
	}
	if len(got)%3 != 0 {
		t.Fatalf("CJK 截断后应只含完整字符(3 字节对齐): got %d", len(got))
	}
	for _, r := range got {
		if r != '中' {
			t.Fatalf("出现非预期字符 %q", r)
		}
	}
}

func TestTruncateLogIOBody_FourByteBoundary(t *testing.T) {
	limit := LogIOMaxBodyBytes()
	// 😀 = 4 字节; 前置一个 ASCII 字节使上限边界错位落入 4 字节字符内部,触发修剪。
	body := append([]byte("a"), []byte(strings.Repeat("😀", limit))...)
	got, truncated := TruncateLogIOBody(body)
	if !truncated {
		t.Fatalf("超过上限的多字节内容应被截断")
	}
	assertNoPartialRune(t, got)
	if len(got) > limit {
		t.Fatalf("截断后不应超过上限: got %d", len(got))
	}
	if !strings.HasPrefix(got, "a") {
		t.Fatalf("前缀内容应保留")
	}
}

// TestLogIOMaxBodyBytes_Configurable 覆盖上限的可配置性与非法值回退:
// 默认 1MB;设置 log_io_max_body_kb=64 等同旧 64KB;<=0 回退默认 1MB。
func TestLogIOMaxBodyBytes_Configurable(t *testing.T) {
	orig := viper.Get("log_io_max_body_kb")
	t.Cleanup(func() { viper.Set("log_io_max_body_kb", orig) })

	viper.Set("log_io_max_body_kb", nil)
	if got := LogIOMaxBodyBytes(); got != LogIODefaultMaxBodyKB*1024 {
		t.Fatalf("默认应为 1MB: got %d want %d", got, LogIODefaultMaxBodyKB*1024)
	}

	viper.Set("log_io_max_body_kb", 64)
	if got := LogIOMaxBodyBytes(); got != 64*1024 {
		t.Fatalf("配置 64 应等同旧 64KB: got %d want %d", got, 64*1024)
	}

	for _, bad := range []int{0, -1} {
		viper.Set("log_io_max_body_kb", bad)
		if got := LogIOMaxBodyBytes(); got != LogIODefaultMaxBodyKB*1024 {
			t.Fatalf("非法值 %d 应回退默认 1MB: got %d", bad, got)
		}
	}
}

func TestTruncateLogIOBody_EmptyAndNil(t *testing.T) {
	for _, body := range [][]byte{nil, {}} {
		got, truncated := TruncateLogIOBody(body)
		if truncated {
			t.Fatalf("空输入不应被截断")
		}
		if got != "" {
			t.Fatalf("空输入应返回空串: got %q", got)
		}
	}
}
