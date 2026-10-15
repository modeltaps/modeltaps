package controller

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/common"
)

// errOAuthCodeRejected marks a token endpoint answer that means the authorization code can no
// longer be used (invalid_grant), as opposed to a network or server failure.
var errOAuthCodeRejected = errors.New("authorization code was rejected")

// oauthPaste is what an admin pasted back from the provider's redirect: a full callback URL, its
// query string, a `code#state` pair (shown by the Claude callback page) or a bare code.
type oauthPaste struct {
	Code             string
	State            string
	Error            string
	ErrorDescription string
}

// parseOAuthPaste extracts the code, state and error from pasted input. It returns
// errOAuthPasteNoCode when no code or error can be found, for example when the authorization
// page link itself was pasted instead of the address shown after approval.
func parseOAuthPaste(input string) (oauthPaste, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return oauthPaste{}, errOAuthPasteNoCode
	}

	var query string
	switch {
	case strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://"):
		u, err := url.Parse(s)
		if err != nil {
			return oauthPaste{}, errOAuthPasteNoCode
		}
		query = u.RawQuery
		// Some providers put the parameters in the fragment.
		if !strings.Contains(query, "code=") && !strings.Contains(query, "error=") {
			query = u.Fragment
		}
	case strings.Contains(s, "code=") || strings.Contains(s, "error="):
		query = strings.TrimPrefix(s, "?")
	default:
		code, state, _ := strings.Cut(s, "#")
		code = strings.TrimSpace(code)
		if len(code) < 10 || strings.ContainsAny(code, " \t\r\n") {
			return oauthPaste{}, errOAuthPasteNoCode
		}
		return oauthPaste{Code: code, State: strings.TrimSpace(state)}, nil
	}

	values, err := url.ParseQuery(query)
	if err != nil {
		return oauthPaste{}, errOAuthPasteNoCode
	}
	p := oauthPaste{
		Code:             strings.TrimSpace(values.Get("code")),
		State:            strings.TrimSpace(values.Get("state")),
		Error:            strings.TrimSpace(values.Get("error")),
		ErrorDescription: truncateErrorDescription(strings.TrimSpace(values.Get("error_description"))),
	}
	if p.Code == "" && p.Error == "" {
		return oauthPaste{}, errOAuthPasteNoCode
	}
	return p, nil
}

var errOAuthPasteNoCode = errors.New("no authorization code found in the pasted text")

// checkOAuthPaste validates pasted input against the login session's state and returns the
// authorization code, or an *common.APIError the console can translate.
func checkOAuthPaste(input, sessionState string) (string, *common.APIError) {
	p, err := parseOAuthPaste(input)
	if err != nil {
		return "", common.NewAPIError(common.APICodeChannelOAuthInvalidInput,
			"No authorization code found. Paste the full address shown in the browser after you approve access.", nil)
	}
	// A missing state is accepted (a bare code was pasted); a different one never is.
	if p.State != "" && p.State != sessionState {
		return "", common.NewAPIError(common.APICodeChannelOAuthStateMismatch,
			"This callback belongs to a different login attempt. Use the address from the authorization page opened by this dialog.", nil)
	}
	if p.Error != "" {
		reason := p.Error
		if p.ErrorDescription != "" {
			reason += " (" + p.ErrorDescription + ")"
		}
		return "", oauthAPIError(common.APICodeChannelOAuthAccessDenied, "The provider did not grant access", reason)
	}
	return p.Code, nil
}

// oauthAPIError builds a coded failure whose message ends with the provider's reason, which also
// goes in params.reason so the console can show it under the translated text.
func oauthAPIError(code, message, reason string) *common.APIError {
	if reason == "" {
		return common.NewAPIError(code, message, nil)
	}
	return common.NewAPIError(code, message+": "+reason, map[string]any{"reason": reason})
}

// respondOAuthError answers a channel OAuth request with a coded failure (HTTP 200, like the
// other channel OAuth answers).
func respondOAuthError(c *gin.Context, err *common.APIError) {
	common.APIRespondWithError(c, http.StatusOK, err)
}

// isOAuthCodeRejected reports whether a token endpoint answer means the code itself is unusable.
func isOAuthCodeRejected(status int, body []byte) bool {
	return (status == http.StatusBadRequest || status == http.StatusUnauthorized) &&
		strings.Contains(string(body), "invalid_grant")
}
