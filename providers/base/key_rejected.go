package base

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/modeltaps/modeltaps/types"
)

// ErrKeyRejected marks an upstream refusing the channel's key (HTTP 401 or 403) on a background
// read such as the balance or the account cost report. Asking again with the same key only adds
// failed attempts, which some upstreams count toward locking the whole account, so the scheduled
// jobs stop asking until the key changes and report the refusal instead.
var ErrKeyRejected = errors.New("the upstream refused the key")

type keyRejectedError struct{ msg string }

func (e *keyRejectedError) Error() string        { return e.msg }
func (e *keyRejectedError) Is(target error) bool { return target == ErrKeyRejected }

// KeyRejected is an ErrKeyRejected error with its own message.
func KeyRejected(msg string) error {
	return &keyRejectedError{msg: msg}
}

// IsKeyRejectedStatus reports whether an HTTP status means the upstream refused the key.
func IsKeyRejectedStatus(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusForbidden
}

// UpstreamReadError turns a failed background read into an error: a refused key becomes an
// ErrKeyRejected error (with rejectedMsg when given), anything else keeps the upstream's message.
func UpstreamReadError(errWithCode *types.OpenAIErrorWithStatusCode, rejectedMsg string) error {
	if errWithCode == nil {
		return nil
	}
	msg := errWithCode.OpenAIError.Message
	if !errWithCode.LocalError && IsKeyRejectedStatus(errWithCode.StatusCode) {
		if rejectedMsg != "" {
			return KeyRejected(rejectedMsg)
		}
		return KeyRejected(fmt.Sprintf("the upstream refused the key (HTTP %d): %s", errWithCode.StatusCode, msg))
	}
	return errors.New(msg)
}
