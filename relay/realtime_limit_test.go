package relay

import "testing"

func TestRealtimeSessionLimiter(t *testing.T) {
	l := &realtimeSessionLimiter{perUser: map[int]int{}}

	r1, ok := l.acquire(1, 3, 2)
	if !ok {
		t.Fatal("first session refused")
	}
	r2, ok := l.acquire(1, 3, 2)
	if !ok {
		t.Fatal("second session refused")
	}
	if _, ok := l.acquire(1, 3, 2); ok {
		t.Fatal("third session of the same user accepted over the per-user limit")
	}
	r3, ok := l.acquire(2, 3, 2)
	if !ok {
		t.Fatal("another user refused below the node limit")
	}
	if _, ok := l.acquire(3, 3, 2); ok {
		t.Fatal("session accepted over the node limit")
	}

	r1()
	r1() // releasing twice must not free a second slot
	if l.total != 2 || l.perUser[1] != 1 {
		t.Fatalf("after release: total=%d user1=%d, want 2 and 1", l.total, l.perUser[1])
	}
	if _, ok := l.acquire(1, 3, 2); !ok {
		t.Fatal("slot not reusable after release")
	}
	r2()
	r3()
}

func TestRealtimeSessionLimiterUnlimited(t *testing.T) {
	l := &realtimeSessionLimiter{perUser: map[int]int{}}
	for i := 0; i < 50; i++ {
		if _, ok := l.acquire(1, 0, 0); !ok {
			t.Fatalf("session %d refused with no limits", i)
		}
	}
}
