package model

import "testing"

// 接口能力序列化按词表定序，词表外的值丢弃，空集合是空串（未设置）。
func TestModelEndpointsJSON(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{[]string{"responses", "chat"}, `["chat","responses"]`},
		{[]string{"images", "nope"}, `["images"]`},
		{[]string{"realtime", "chat"}, `["chat","realtime"]`},
		{[]string{"nope"}, ""},
		{nil, ""},
	}
	for _, c := range cases {
		if got := ModelEndpointsJSON(c.in); got != c.want {
			t.Errorf("ModelEndpointsJSON(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

// 校验：空串（未设置）合法，词表外的值与非数组非法。
func TestIsValidModelEndpoints(t *testing.T) {
	valid := []string{"", "[]", `["chat"]`, `["chat","responses","images","audio.speech"]`, `["realtime"]`}
	for _, raw := range valid {
		if !IsValidModelEndpoints(raw) {
			t.Errorf("IsValidModelEndpoints(%q) = false, want true", raw)
		}
	}
	invalid := []string{`["chats"]`, `{"chat":true}`, "chat"}
	for _, raw := range invalid {
		if IsValidModelEndpoints(raw) {
			t.Errorf("IsValidModelEndpoints(%q) = true, want false", raw)
		}
	}
}

// UnknownModelEndpoints 报出词表外的原始取值（去重），供种子等场景在归一化前报错。
func TestUnknownModelEndpoints(t *testing.T) {
	if got := UnknownModelEndpoints([]string{"chat", "realtime", "audio.speech"}); len(got) != 0 {
		t.Errorf("词表内取值不应报未知, got %v", got)
	}
	got := UnknownModelEndpoints([]string{"chat", "videos", "videos", "audio"})
	if len(got) != 2 || got[0] != "videos" || got[1] != "audio" {
		t.Errorf("UnknownModelEndpoints = %v, want [videos audio]", got)
	}
}

// 由模态推导接口能力：输入含 text → chat+responses；输出含 image 且属图像生成名单 → images；
// 输出只有 audio → 仅 audio.speech；对话模型输出含 audio 不推 audio.speech；推不出则空串。
func TestModelEndpointsFromModalities(t *testing.T) {
	cases := []struct {
		name   string
		input  []string
		output []string
		want   string
	}{
		{"gpt-4o", []string{"text", "image"}, []string{"text"}, `["chat","responses"]`},
		{"dall-e-3", []string{"text"}, []string{"image"}, `["chat","responses","images"]`},
		{"gemini-2.5-flash-image-preview", []string{"text"}, []string{"text", "image"}, `["chat","responses"]`},
		{"tts-1", nil, []string{"audio"}, `["audio.speech"]`},
		{"openai/gpt-4o-mini-tts", []string{"text"}, []string{"speech"}, `["audio.speech"]`},
		{"openai/whisper-1", []string{"audio"}, []string{"transcription"}, `["audio.transcription"]`},
		{"gemini-2.5-flash-preview-tts", []string{"text"}, []string{"audio"}, `["audio.speech"]`},
		{"whisper-large-v3", []string{"audio"}, []string{"text"}, `["audio.transcription"]`},
		{"gpt-4o-transcribe", []string{"text", "audio"}, []string{"text"}, `["audio.transcription"]`},
		{"gpt-4o-audio-preview", []string{"text", "audio"}, []string{"text", "audio"}, `["chat","responses"]`},
		{"openai/gpt-audio", []string{"text", "audio"}, []string{"text", "audio"}, `["chat","responses"]`},
		{"lyria-3-pro-preview", []string{"text", "image"}, []string{"text", "audio"}, `["chat","responses"]`},
		{"hexgrad/kokoro-82m", []string{"text"}, []string{"speech"}, `["audio.speech"]`},
		{"kokoro-82m", []string{"text"}, []string{"audio"}, `["audio.speech"]`},
		{"gemini-3.5-live-translate-preview", []string{"audio"}, []string{"audio", "text"}, ""},
		{"gemini-2.5-flash", []string{"text", "image", "audio", "video"}, []string{"text"}, `["chat","responses"]`},
		{"mystery", nil, nil, ""},
	}
	for _, c := range cases {
		if got := ModelEndpointsFromModalities(c.name, c.input, c.output); got != c.want {
			t.Errorf("ModelEndpointsFromModalities(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

// 由接口能力推导任务类型：images → image；chat + 输出含 image → chat_image；其余 chat。
func TestModelModeFromEndpoints(t *testing.T) {
	cases := []struct {
		endpoints string
		output    string
		want      string
	}{
		{`["images"]`, "", ModelModeImage},
		{`["chat","responses","images"]`, `["image"]`, ModelModeImage},
		{`["chat","responses"]`, `["text","image"]`, ModelModeChatImage},
		{`["chat","responses"]`, `["text"]`, ModelModeChat},
		{`["chat"]`, "", ModelModeChat},
		{`["audio.speech"]`, "", ""},
		{`["realtime"]`, "", ""},
		{"", `["image"]`, ""},
		{"not json", "", ""},
	}
	for _, c := range cases {
		if got := ModelModeFromEndpoints(c.endpoints, c.output); got != c.want {
			t.Errorf("ModelModeFromEndpoints(%q,%q) = %q, want %q", c.endpoints, c.output, got, c.want)
		}
	}
}

// EffectiveMode：管理员显式设置的 mode 优先，为空时才由 endpoints 推导。
func TestModelInfoEffectiveMode(t *testing.T) {
	explicit := &ModelInfo{Mode: ModelModeChat, Endpoints: `["images"]`}
	if got := explicit.EffectiveMode(); got != ModelModeChat {
		t.Errorf("explicit mode = %q, want %q", got, ModelModeChat)
	}
	derived := &ModelInfo{Endpoints: `["chat"]`, OutputModalities: `["text","image"]`}
	if got := derived.EffectiveMode(); got != ModelModeChatImage {
		t.Errorf("derived mode = %q, want %q", got, ModelModeChatImage)
	}
	unset := &ModelInfo{}
	if got := unset.EffectiveMode(); got != "" {
		t.Errorf("unset mode = %q, want empty", got)
	}
}
