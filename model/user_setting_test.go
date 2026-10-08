package model

import (
	"encoding/json"
	"testing"

	"github.com/modeltaps/modeltaps/common/database"
)

func strPtr(s string) *string { return &s }

func TestUserSettingDefault(t *testing.T) {
	// 老用户 setting 为 NULL/未设置时，JSONType 零值应回到全空默认值且校验通过
	var value database.JSONType[UserSetting]
	setting := value.Data()
	if setting.Theme != "" || setting.Language != "" {
		t.Errorf("默认值应为全空, got %+v", setting)
	}
	if err := setting.Validate(); err != nil {
		t.Errorf("空默认值应校验通过, got %v", err)
	}
}

func TestUserSettingJSONRoundTrip(t *testing.T) {
	var value database.JSONType[UserSetting]
	value.Set(UserSetting{Theme: "dark", Language: "en_US"})

	data, err := json.Marshal(&value)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var decoded database.JSONType[UserSetting]
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	if got := decoded.Data(); got.Theme != "dark" || got.Language != "en_US" {
		t.Errorf("round-trip 不一致, got %+v", got)
	}

	// omitempty: 全空设置序列化为 {}
	var empty database.JSONType[UserSetting]
	empty.Set(UserSetting{})
	data, err = json.Marshal(&empty)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	if string(data) != "{}" {
		t.Errorf("全空设置应序列化为 {}, got %s", data)
	}
}

func TestUserSettingApplyUpdate(t *testing.T) {
	tests := []struct {
		name   string
		base   UserSetting
		update UserSettingUpdate
		want   UserSetting
	}{
		{"仅更新 theme 保留 language", UserSetting{Theme: "light", Language: "zh_CN"}, UserSettingUpdate{Theme: strPtr("dark")}, UserSetting{Theme: "dark", Language: "zh_CN"}},
		{"仅更新 language 保留 theme", UserSetting{Theme: "light", Language: "zh_CN"}, UserSettingUpdate{Language: strPtr("ja_JP")}, UserSetting{Theme: "light", Language: "ja_JP"}},
		{"同时更新两个字段", UserSetting{}, UserSettingUpdate{Theme: strPtr("system"), Language: strPtr("en_US")}, UserSetting{Theme: "system", Language: "en_US"}},
		{"全 nil 不改动", UserSetting{Theme: "dark", Language: "zh_HK"}, UserSettingUpdate{}, UserSetting{Theme: "dark", Language: "zh_HK"}},
		{"显式空串清除字段", UserSetting{Theme: "dark", Language: "zh_CN"}, UserSettingUpdate{Theme: strPtr("")}, UserSetting{Theme: "", Language: "zh_CN"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setting := tt.base
			setting.ApplyUpdate(tt.update)
			if setting != tt.want {
				t.Errorf("ApplyUpdate(%+v, %+v) = %+v, want %+v", tt.base, tt.update, setting, tt.want)
			}
		})
	}
}

func TestUserSettingValidate(t *testing.T) {
	valid := []UserSetting{
		{},
		{Theme: "system"},
		{Theme: "light"},
		{Theme: "dark", Language: "zh_CN"},
		{Language: "zh_HK"},
		{Language: "en_US"},
		{Language: "ja_JP"},
	}
	for _, s := range valid {
		if err := s.Validate(); err != nil {
			t.Errorf("%+v 应校验通过, got %v", s, err)
		}
	}
	invalid := []UserSetting{
		{Theme: "blue"},
		{Theme: "DARK"},
		{Language: "fr_FR"},
		{Language: "zh-CN"},
	}
	for _, s := range invalid {
		if err := s.Validate(); err == nil {
			t.Errorf("%+v 应校验失败", s)
		}
	}
}
