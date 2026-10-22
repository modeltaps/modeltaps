package model

import (
	"encoding/binary"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/redis"

	"github.com/coocood/freecache"
)

// stickyMemoryCacheBytes bounds the in-process session map used when Redis is off.
// One entry is a short hash key plus a 4-byte channel id, so 16 MB holds well over
// 100k sessions; freecache evicts the oldest entries when it is full.
const stickyMemoryCacheBytes = 16 * 1024 * 1024

var stickyMemory = freecache.NewCache(stickyMemoryCacheBytes)

// stickyMemoryKey keeps the Redis key layout so both stores agree on namespaces.
func stickyMemoryKey(sessionHash string, keyType int) []byte {
	return []byte(redis.GetStickySessionKeyPrefix(keyType) + sessionHash)
}

// stickyLookup returns the channel bound to the session, or 0. It reads Redis when
// Redis is on and the in-process map otherwise, so single-node deployments without
// Redis keep their sessions on one channel too.
func stickyLookup(sessionHash string, keyType int) int {
	if sessionHash == "" {
		return 0
	}
	if config.RedisEnabled {
		channelID, err := redis.GetStickySessionMapping(sessionHash, keyType)
		if err != nil {
			return 0
		}
		return channelID
	}
	value, err := stickyMemory.Get(stickyMemoryKey(sessionHash, keyType))
	if err != nil || len(value) != 4 {
		return 0
	}
	return int(binary.BigEndian.Uint32(value))
}

// stickyBind binds the session to a channel for ttl.
func stickyBind(sessionHash string, channelID int, keyType int, ttl time.Duration) error {
	if sessionHash == "" || channelID <= 0 {
		return nil
	}
	if config.RedisEnabled {
		return redis.SetStickySessionMapping(sessionHash, channelID, keyType, ttl)
	}
	value := make([]byte, 4)
	binary.BigEndian.PutUint32(value, uint32(channelID))
	return stickyMemory.Set(stickyMemoryKey(sessionHash, keyType), value, int(ttl.Seconds()))
}

// stickyForget removes the session binding.
func stickyForget(sessionHash string, keyType int) {
	if sessionHash == "" {
		return
	}
	if config.RedisEnabled {
		_ = redis.DeleteStickySessionMapping(sessionHash, keyType)
		return
	}
	stickyMemory.Del(stickyMemoryKey(sessionHash, keyType))
}
