package controller

import "github.com/modeltaps/modeltaps/common"

// Coded errors shared by several handlers (see common/api_error.go).
var (
	errInvalidParams           = common.NewAPIError(common.APICodeInvalidParams, "Invalid parameters", nil)
	errInvalidEmail            = common.NewAPIError(common.APICodeInvalidEmail, "Invalid email format", nil)
	errSessionSaveFailed       = common.NewAPIError(common.APICodeSessionSaveFailed, "Unable to save session, please try again", nil)
	errVerificationCodeInvalid = common.NewAPIError(common.APICodeVerificationCodeInvalid, "Verification code is incorrect or has expired", nil)
)

// emailSendFailed keeps the mail server's own text in message; the console shows a generic line.
func emailSendFailed(err error) error {
	return common.NewAPIError(common.APICodeEmailSendFailed, err.Error(), nil)
}
