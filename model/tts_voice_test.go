package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/types"
)

func TestInferTTSVoice(t *testing.T) {
	cases := []types.TTSVoice{
		{ID: "zf_xiaoxiao", Language: "zh", Gender: "female"},
		{ID: "am_adam", Language: "en", Gender: "male"},
		{ID: "bf_emma", Language: "en", Gender: "female"},
		{ID: "ef_dora", Language: "es", Gender: "female"},
		{ID: "hm_omega", Language: "hi", Gender: "male"},
		{ID: "jf_alpha", Language: "ja", Gender: "female"},
		{ID: "pm_alex", Language: "pt", Gender: "male"},
		{ID: "aura-2-agathe-fr", Language: "fr"},
		{ID: "aura-2-ama-ja", Language: "ja"},
		{ID: "flux-alexis-en", Language: "en"},
		{ID: "en-US-Harper:MAI-Voice-2", Language: "en"},
		{ID: "es-MX-Valeria:MAI-Voice-2", Language: "es"},
		{ID: "English_radiant_girl", Language: "en", Gender: "female"},
		{ID: "English_Upbeat_Woman", Language: "en", Gender: "female"},
		{ID: "English_ManWithDeepVoice", Language: "en", Gender: "male"},
		{ID: "English_Deep-VoicedGentleman", Language: "en", Gender: "male"},
		{ID: "English_compelling_lady1", Language: "en", Gender: "female"},
		{ID: "English_ImposingManner", Language: "en"},
		{ID: "English_MaturePartner", Language: "en"},
		{ID: "Chinese (Mandarin)_Reliable_Executive", Language: "zh"},
		{ID: "en_paul_sad", Language: "en"},
		{ID: "gb_jane_neutral", Language: "en"},
		{ID: "fr_marie_happy", Language: "fr"},
		{ID: "female-chengshu", Gender: "female"},
		{ID: "presenter_male", Gender: "male"},
		{ID: "Kore"},
		{ID: "longanhuan_v3.6"},
		{ID: "conversational_a"},
	}
	for _, want := range cases {
		if got := InferTTSVoice(want.ID); got != want {
			t.Errorf("InferTTSVoice(%q) = %+v, want %+v", want.ID, got, want)
		}
	}
}

func TestTTSVoicesFromIDsSkipsEmpty(t *testing.T) {
	got := TTSVoicesFromIDs([]string{"zf_xiaoxiao", "", "Kore"})
	if len(got) != 2 || got[0].ID != "zf_xiaoxiao" || got[1].ID != "Kore" {
		t.Fatalf("got %+v", got)
	}
}

// 同步写入 supported_voices，同步值为空时保留已有音色；别名随主名；后台编辑不清空音色。
func TestModelSupportedVoicesSyncAndAdminEdit(t *testing.T) {
	setupModelInfoTestDB(t)
	if err := UpsertModelInfos([]*ModelInfo{
		{Model: "hexgrad/kokoro-82m", SupportedVoices: `["af_alloy","zf_xiaoxiao"]`},
	}, ModelInfoSourceOpenRouter); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := UpsertModelInfos([]*ModelInfo{{Model: "hexgrad/kokoro-82m", Name: "Kokoro"}}, ModelInfoSourceOpenRouter); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := DB.Create(&ModelInfo{Model: "kokoro", AliasOf: "hexgrad/kokoro-82m"}).Error; err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"hexgrad/kokoro-82m", "kokoro"} {
		got := ModelSupportedVoices(name)
		if len(got) != 2 || got[0] != "af_alloy" || got[1] != "zf_xiaoxiao" {
			t.Fatalf("ModelSupportedVoices(%q) = %v", name, got)
		}
	}
	if got := ModelSupportedVoices("unknown"); len(got) != 0 {
		t.Fatalf("unknown model should have no voices, got %v", got)
	}

	row, err := GetModelInfoByModel("hexgrad/kokoro-82m")
	if err != nil {
		t.Fatal(err)
	}
	row.SupportedVoices = ""
	row.Description = "edited"
	if err := UpdateModelInfo(row); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got := ModelSupportedVoices("hexgrad/kokoro-82m"); len(got) != 2 {
		t.Fatalf("admin edit must not clear supported_voices, got %v", got)
	}
}
