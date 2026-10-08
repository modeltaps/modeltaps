package config

import (
	"reflect"
	"testing"
)

// 验证 scope 过滤逻辑:GetAll / GetAllNonSecret / PublicOptions
func TestOptionManagerScopeFiltering(t *testing.T) {
	cm := NewOptionManager()

	secretVal := "s3cret"
	adminVal := "admin-only"
	boolVal := true
	floatVal := 1.5
	aliasVal := "aliased"

	cm.RegisterString("SecretKey", &secretVal, ScopeSecret)
	cm.RegisterString("AdminKey", &adminVal, ScopeAdmin)
	cm.RegisterBool("PublicBool", &boolVal, ScopePublic)
	cm.RegisterFloat("PublicFloat", &floatVal, ScopePublic)
	cm.RegisterString("PublicAliased", &aliasVal, ScopePublic, PublicSpec{StatusKey: "public_aliased"})
	cm.RegisterCustom("PublicCustom", func() string { return "getter-value" }, func(string) error { return nil }, "", ScopePublic, PublicSpec{StatusValue: func() any { return []string{"a", "b"} }})

	// GetAll 包含全部 key(含 secret)
	all := cm.GetAll()
	if len(all) != 6 {
		t.Fatalf("GetAll expected 6 keys, got %d: %v", len(all), all)
	}

	// GetAllNonSecret 排除 ScopeSecret,保留 admin 与 public
	nonSecret := cm.GetAllNonSecret()
	if _, ok := nonSecret["SecretKey"]; ok {
		t.Errorf("GetAllNonSecret must exclude ScopeSecret key")
	}
	for _, k := range []string{"AdminKey", "PublicBool", "PublicFloat", "PublicAliased", "PublicCustom"} {
		if _, ok := nonSecret[k]; !ok {
			t.Errorf("GetAllNonSecret missing key %q", k)
		}
	}
	if len(nonSecret) != 5 {
		t.Errorf("GetAllNonSecret expected 5 keys, got %d", len(nonSecret))
	}

	// PublicOptions 仅含 ScopePublic
	public := cm.PublicOptions()
	if len(public) != 4 {
		t.Fatalf("PublicOptions expected 4 keys, got %d: %v", len(public), public)
	}
	if _, ok := public["AdminKey"]; ok {
		t.Errorf("PublicOptions must exclude ScopeAdmin key")
	}
	if _, ok := public["SecretKey"]; ok {
		t.Errorf("PublicOptions must exclude ScopeSecret key")
	}

	// 默认 statusKey = 注册 key,且保留原始类型
	if v, ok := public["PublicBool"]; !ok || v != true {
		t.Errorf("PublicBool expected typed bool true, got %#v", v)
	}
	if v, ok := public["PublicFloat"]; !ok || v != 1.5 {
		t.Errorf("PublicFloat expected typed float 1.5, got %#v", v)
	}

	// StatusKey 别名生效,原 key 不出现
	if _, ok := public["PublicAliased"]; ok {
		t.Errorf("aliased option must not appear under registration key")
	}
	if v, ok := public["public_aliased"]; !ok || v != "aliased" {
		t.Errorf("public_aliased expected %q, got %#v", "aliased", v)
	}

	// StatusValue 覆盖生效(类型化取值优先于字符串 getter)
	if v, ok := public["PublicCustom"]; !ok || !reflect.DeepEqual(v, []string{"a", "b"}) {
		t.Errorf("PublicCustom expected []string{a,b}, got %#v", v)
	}
}
