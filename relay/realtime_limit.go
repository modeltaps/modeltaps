package relay

import (
	"sync"

	"github.com/modeltaps/modeltaps/common/utils"
)

// Limits for realtime WebSocket sessions. A WebSocket outlives the HTTP server's timeouts once it
// is hijacked, so without these one client could hold any number of sessions open or send
// frames of any size.
//
//   - realtime.max_message_bytes: largest client frame accepted (default 15 MiB, the size
//     OpenAI allows for one input_audio_buffer.append event). A larger frame closes the session.
//   - realtime.max_sessions: open sessions across the whole node (default 1000, 0 = no limit).
//   - realtime.max_sessions_per_user: open sessions per user (default 10, 0 = no limit).
const (
	defaultRealtimeMaxMessageBytes    = 15 << 20
	defaultRealtimeMaxSessions        = 1000
	defaultRealtimeMaxSessionsPerUser = 10
)

func realtimeMaxMessageBytes() int64 {
	n := utils.GetOrDefault("realtime.max_message_bytes", defaultRealtimeMaxMessageBytes)
	if n <= 0 {
		return defaultRealtimeMaxMessageBytes
	}
	return int64(n)
}

type realtimeSessionLimiter struct {
	mu      sync.Mutex
	total   int
	perUser map[int]int
}

var realtimeSessions = &realtimeSessionLimiter{perUser: map[int]int{}}

// acquire reserves a session slot for the user. It returns a release func and true, or false
// when the node or the user is at the limit.
func (l *realtimeSessionLimiter) acquire(userId, maxTotal, maxPerUser int) (func(), bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if maxTotal > 0 && l.total >= maxTotal {
		return nil, false
	}
	if maxPerUser > 0 && l.perUser[userId] >= maxPerUser {
		return nil, false
	}
	l.total++
	l.perUser[userId]++
	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			defer l.mu.Unlock()
			l.total--
			if l.perUser[userId] <= 1 {
				delete(l.perUser, userId)
			} else {
				l.perUser[userId]--
			}
		})
	}, true
}

func acquireRealtimeSession(userId int) (func(), bool) {
	return realtimeSessions.acquire(userId,
		utils.GetOrDefault("realtime.max_sessions", defaultRealtimeMaxSessions),
		utils.GetOrDefault("realtime.max_sessions_per_user", defaultRealtimeMaxSessionsPerUser))
}
