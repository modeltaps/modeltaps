package config

import (
	"net"
	"reflect"
	"testing"
)

func TestParseTrustedProxies(t *testing.T) {
	cases := []struct {
		name string
		raw  []string
		want []string
	}{
		{"空配置不信任任何代理", nil, nil},
		{"全空白项被丢弃", []string{"", "  ", ","}, nil},
		{"YAML 列表原样保留", []string{"10.0.0.0/8", "::1"}, []string{"10.0.0.0/8", "::1"}},
		{"逗号分隔的环境变量写法", []string{"10.0.0.0/8, 172.16.0.0/12 ,192.168.1.1"}, []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.1.1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ParseTrustedProxies(tc.raw); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ParseTrustedProxies(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestParseCIDRList(t *testing.T) {
	got := ParseCIDRList([]string{"10.0.0.5, 192.168.0.0/16", "bad-entry", "2001:0db8::1", "300.1.1.1/8"}, "test")
	var nets []string
	for _, n := range got {
		nets = append(nets, n.String())
	}
	want := []string{"10.0.0.5/32", "192.168.0.0/16", "2001:db8::1/128"}
	if !reflect.DeepEqual(nets, want) {
		t.Fatalf("ParseCIDRList = %q, want %q", nets, want)
	}
	if !got[2].Contains(net.ParseIP("2001:db8::1")) {
		t.Fatal("IPv6 entry must match regardless of written form")
	}
}
