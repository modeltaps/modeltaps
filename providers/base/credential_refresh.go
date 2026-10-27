package base

import (
	"context"
	"time"

	"github.com/modeltaps/modeltaps/common/cache"
)

// CredentialRefresher is implemented by account channels with OAuth credentials (Codex,
// Claude Code, Gemini CLI, Antigravity). The background task calls it so that credentials of
// idle channels are refreshed before they expire, not only when a request arrives.
type CredentialRefresher interface {
	// RefreshCredentialsAhead refreshes and saves the credentials when they expire within
	// ahead. It reports whether a refresh happened.
	RefreshCredentialsAhead(ctx context.Context, ahead time.Duration) (bool, error)
}

// RefreshOAuthAhead is the shared body of RefreshCredentialsAhead: credentials without a
// refresh token are left alone, credentials that stay valid beyond ahead are skipped, and a
// refreshed token replaces the cached one.
func RefreshOAuthAhead(ahead time.Duration, hasRefreshToken bool, expiresAt time.Time,
	refresh func() error, save func() error, cacheKey string) (bool, error) {
	if !hasRefreshToken {
		return false, nil
	}
	if !expiresAt.IsZero() && time.Until(expiresAt) > ahead {
		return false, nil
	}
	if err := refresh(); err != nil {
		return false, err
	}
	if err := save(); err != nil {
		return true, err
	}
	_ = cache.DeleteCache(cacheKey)
	return true, nil
}
