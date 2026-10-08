package model

import "testing"

// mappingTargetFor：从单条渠道 model_mapping JSON 里取 modelName 的映射目标。
// "+" 前缀表示按传入(别名)计费，此时别名用自己的价，不解析目标 → ok=false。
func TestMappingTargetFor(t *testing.T) {
	cases := []struct {
		mapping   string
		model     string
		wantTgt   string
		wantOk    bool
	}{
		{`{"opus-latest":"claude-opus-4.8"}`, "opus-latest", "claude-opus-4.8", true},
		{`{"opus-latest":"+claude-opus-4.8"}`, "opus-latest", "", false}, // 按传入计费→不解析
		{`{"a":"b"}`, "opus-latest", "", false},                          // 无此 key
		{``, "opus-latest", "", false},                                   // 空
		{`not-json`, "opus-latest", "", false},                           // 非法
	}
	for _, c := range cases {
		tgt, ok := mappingTargetFor(c.mapping, c.model)
		if tgt != c.wantTgt || ok != c.wantOk {
			t.Errorf("mappingTargetFor(%q,%q)=(%q,%v) want (%q,%v)", c.mapping, c.model, tgt, ok, c.wantTgt, c.wantOk)
		}
	}
}
