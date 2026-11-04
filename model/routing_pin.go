package model

import (
	"strings"
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/redis"
)

// 路由决策的会话固定（pin）：同一会话首次决策后把「标签 + 目标模型」存起来，后续轮次直接复用，
// 不再分类，也就不会中途换模型。存储与渠道会话亲和同源：Redis 开启时放 Redis，否则放进程内
// freecache；key 前缀独立，避免和渠道亲和的 key 混在一起。

// routingPinKeyPrefix 路由固定记录的 key 前缀。
const routingPinKeyPrefix = "sticky_route:"

// routingPinSeparator 分隔标签与目标模型；标签与模型名都不含 NUL。
const routingPinSeparator = "\x00"

// RoutingPin 一条会话固定记录。
type RoutingPin struct {
	Label       string
	TargetModel string
}

func (p RoutingPin) encode() string {
	return p.Label + routingPinSeparator + p.TargetModel
}

func decodeRoutingPin(value string) (RoutingPin, bool) {
	label, target, ok := strings.Cut(value, routingPinSeparator)
	if !ok || target == "" {
		return RoutingPin{}, false
	}
	return RoutingPin{Label: label, TargetModel: target}, true
}

// routingPinMemoryMu 让进程内存储的「不存在才写」是原子的；freecache 本身没有 SetNX。
var routingPinMemoryMu sync.Mutex

func routingPinKey(sessionHash string) string {
	return routingPinKeyPrefix + sessionHash
}

// LookupRoutingPin 读取会话的固定记录，没有返回 ok=false。
func LookupRoutingPin(sessionHash string) (RoutingPin, bool) {
	if sessionHash == "" {
		return RoutingPin{}, false
	}
	if config.RedisEnabled {
		value, err := redis.RedisGet(routingPinKey(sessionHash))
		if err != nil || value == "" {
			return RoutingPin{}, false
		}
		return decodeRoutingPin(value)
	}
	value, err := stickyMemory.Get([]byte(routingPinKey(sessionHash)))
	if err != nil {
		return RoutingPin{}, false
	}
	return decodeRoutingPin(string(value))
}

// BindRoutingPin 写入固定记录，已存在时以先写入者为准并返回它（并发首批请求各自分类时，
// 后来者改用先写入的决策，保证同一会话只有一个目标）。
func BindRoutingPin(sessionHash string, pin RoutingPin, ttl time.Duration) (RoutingPin, error) {
	if sessionHash == "" || pin.TargetModel == "" || ttl <= 0 {
		return pin, nil
	}
	key := routingPinKey(sessionHash)
	if config.RedisEnabled {
		set, err := redis.RedisSetNX(key, pin.encode(), ttl)
		if err != nil {
			return pin, err
		}
		if set {
			return pin, nil
		}
		if existing, ok := LookupRoutingPin(sessionHash); ok {
			return existing, nil
		}
		return pin, nil
	}
	routingPinMemoryMu.Lock()
	defer routingPinMemoryMu.Unlock()
	if value, err := stickyMemory.Get([]byte(key)); err == nil {
		if existing, ok := decodeRoutingPin(string(value)); ok {
			return existing, nil
		}
	}
	return pin, stickyMemory.Set([]byte(key), []byte(pin.encode()), int(ttl.Seconds()))
}

// RenewRoutingPin 按最后一次请求续期，长会话不会在中途失效后换模型。
func RenewRoutingPin(sessionHash string, pin RoutingPin, ttl time.Duration) {
	if sessionHash == "" || ttl <= 0 {
		return
	}
	key := routingPinKey(sessionHash)
	if config.RedisEnabled {
		_ = redis.RedisSet(key, pin.encode(), ttl)
		return
	}
	_ = stickyMemory.Set([]byte(key), []byte(pin.encode()), int(ttl.Seconds()))
}

// ForgetRoutingPin 删除固定记录（目标已不在策略里、策略被改时）。
func ForgetRoutingPin(sessionHash string) {
	if sessionHash == "" {
		return
	}
	key := routingPinKey(sessionHash)
	if config.RedisEnabled {
		_ = redis.RedisDel(key)
		return
	}
	stickyMemory.Del([]byte(key))
}

// ClearStickyMemory 清空进程内的会话存储（路由固定与渠道亲和共用）。供测试隔离用例。
func ClearStickyMemory() {
	stickyMemory.Clear()
}
