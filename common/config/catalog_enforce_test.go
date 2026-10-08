package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestCatalogEnforceHiddenFallback(t *testing.T) {
	cases := []struct {
		name string
		set  map[string]any
		want bool
	}{
		{"两者都未设置时默认开", nil, true},
		{"新键优先", map[string]any{"catalog.enforce_hidden": false, "catalog.enforce_published": true}, false},
		{"新键开启", map[string]any{"catalog.enforce_hidden": true}, true},
		{"未设新键时沿用旧键关闭", map[string]any{"catalog.enforce_published": false}, false},
		{"未设新键时沿用旧键开启", map[string]any{"catalog.enforce_published": true}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			for k, v := range tc.set {
				viper.Set(k, v)
			}
			if got := CatalogEnforceHidden(); got != tc.want {
				t.Fatalf("CatalogEnforceHidden() = %v, want %v", got, tc.want)
			}
		})
	}
}
