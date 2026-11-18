package safefetch

import (
	"context"
	"testing"
)

func TestFetchRejectsABadProxy(t *testing.T) {
	if _, err := Fetch(context.Background(), "https://example.com/a.png", Options{Proxy: "ftp://proxy.example.com"}); err == nil {
		t.Fatal("an unsupported proxy scheme must fail")
	}
}
