package common

import (
	"errors"

	"github.com/gin-gonic/gin"
)

// Admin JSON APIs answer {success, message}. A response whose message is meant to be shown to
// a person may also carry `code`, a stable snake_case identifier, and `params`, the values the
// text was built from. The web console translates known codes and falls back to message, which
// stays English for API clients and logs. Codes are part of the API: never rename or reuse one.
// Raw upstream or database text belongs in a param (or in message alone), never in a code.
const (
	APICodeRateLimitExceeded = "rate_limit_exceeded"

	APICodeChannelTestSucceeded          = "channel_test_succeeded"
	APICodeChannelTestFailed             = "channel_test_failed"
	APICodeChannelTestFailedAndDisabled  = "channel_test_failed_disabled"
	APICodeChannelNoTestModel            = "channel_no_test_model"
	APICodeInvalidChannelID              = "invalid_channel_id"
	APICodeChannelNotFound               = "channel_not_found"
	APICodeChannelKeyRequired            = "channel_key_required"
	APICodeChannelProviderNotFound       = "channel_provider_not_found"
	APICodeChannelModelListUnsupported   = "channel_model_list_unsupported"
	APICodeChannelPricingSyncUnsupported = "channel_pricing_sync_unsupported"
	APICodeChannelIDsRequired            = "channel_ids_required"
	APICodeChannelUserGroupRequired      = "channel_user_group_required"
	APICodeChannelModelRequired          = "channel_model_required"
	APICodeChannelTagRequired            = "channel_tag_required"
	APICodeChannelTagTypeMismatch        = "channel_tag_type_mismatch"
	APICodeChannelTagInvalidSettingType  = "channel_tag_invalid_setting_type"
	APICodeChannelTagInvalidStatus       = "channel_tag_invalid_status"

	// Channel OAuth (ClaudeCode / Codex). The provider's own text goes in params.reason.
	APICodeChannelOAuthSessionExpired    = "channel_oauth_session_expired"
	APICodeChannelOAuthStateMismatch     = "channel_oauth_state_mismatch"
	APICodeChannelOAuthAccessDenied      = "channel_oauth_access_denied"
	APICodeChannelOAuthInvalidInput      = "channel_oauth_invalid_input"
	APICodeChannelOAuthCodeRejected      = "channel_oauth_code_rejected"
	APICodeChannelOAuthExchangeFailed    = "channel_oauth_exchange_failed"
	APICodeChannelOAuthDeviceAuthOff     = "channel_oauth_device_auth_disabled"
	APICodeChannelOAuthDeviceCodeExpired = "channel_oauth_device_code_expired"
	APICodeChannelOAuthDeviceAuthFailed  = "channel_oauth_device_auth_failed"
)

// APIError is an error with a stable code. Returning one from a model or controller helper lets
// APIRespondWithError put the code and params in the response; Error() is the English message.
type APIError struct {
	Code    string
	Message string
	Params  map[string]any
}

func (e *APIError) Error() string {
	return e.Message
}

func NewAPIError(code string, message string, params map[string]any) *APIError {
	return &APIError{Code: code, Message: message, Params: params}
}

// APIErrorBody builds the failure body {success:false, message[, code][, params]}.
func APIErrorBody(err error) gin.H {
	body := gin.H{
		"success": false,
		"message": err.Error(),
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		WithAPICode(body, apiErr.Code, apiErr.Params)
	}
	return body
}

// WithAPICode adds code and, when there are any, params to a response body and returns it.
func WithAPICode(body gin.H, code string, params map[string]any) gin.H {
	if code != "" {
		body["code"] = code
	}
	if len(params) > 0 {
		body["params"] = params
	}
	return body
}

// APIRespondWithCode answers a failure with a stable code.
func APIRespondWithCode(c *gin.Context, status int, code string, message string, params map[string]any) {
	APIRespondWithError(c, status, NewAPIError(code, message, params))
}
