package model

import (
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
)

// TestRoutingPinStore 进程内存储：写入 / 读取 / 续期 / 删除，已存在时 Bind 返回先写入者
func TestRoutingPinStore(t *testing.T) {
	prev := config.RedisEnabled
	config.RedisEnabled = false
	t.Cleanup(func() { config.RedisEnabled = prev })

	hash := "pin-test-" + time.Now().Format(time.RFC3339Nano)
	if _, ok := LookupRoutingPin(hash); ok {
		t.Fatalf("空会话不应命中")
	}
	first := RoutingPin{Label: "small", TargetModel: "cheap"}
	bound, err := BindRoutingPin(hash, first, time.Minute)
	if err != nil || bound != first {
		t.Fatalf("首次写入应成功并返回自身: %+v err=%v", bound, err)
	}
	second := RoutingPin{Label: "large", TargetModel: "big"}
	bound, err = BindRoutingPin(hash, second, time.Minute)
	if err != nil || bound != first {
		t.Fatalf("已存在时应返回先写入者: %+v err=%v", bound, err)
	}
	if got, ok := LookupRoutingPin(hash); !ok || got != first {
		t.Fatalf("读取应得到先写入者: %+v ok=%v", got, ok)
	}
	RenewRoutingPin(hash, first, time.Minute)
	if got, ok := LookupRoutingPin(hash); !ok || got != first {
		t.Fatalf("续期后应仍可读: %+v ok=%v", got, ok)
	}
	ForgetRoutingPin(hash)
	if _, ok := LookupRoutingPin(hash); ok {
		t.Fatalf("删除后不应命中")
	}

	// 空哈希 / 空目标 / 非正 TTL 都是 no-op。
	if _, err := BindRoutingPin("", first, time.Minute); err != nil {
		t.Fatalf("空哈希应忽略")
	}
	if _, err := BindRoutingPin(hash, RoutingPin{}, time.Minute); err != nil {
		t.Fatalf("空目标应忽略")
	}
	if _, ok := LookupRoutingPin(hash); ok {
		t.Fatalf("no-op 不应写入")
	}
	if _, ok := decodeRoutingPin("no-separator"); ok {
		t.Fatalf("没有分隔符的值应视为无效")
	}
}
