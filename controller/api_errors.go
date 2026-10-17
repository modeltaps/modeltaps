package controller

import (
	"errors"

	"github.com/modeltaps/modeltaps/common"

	"gorm.io/gorm"
)

// Coded errors shared by several handlers (see common/api_error.go).
var (
	errInvalidParams           = common.NewAPIError(common.APICodeInvalidParams, "Invalid parameters", nil)
	errInvalidEmail            = common.NewAPIError(common.APICodeInvalidEmail, "Invalid email format", nil)
	errSessionSaveFailed       = common.NewAPIError(common.APICodeSessionSaveFailed, "Unable to save session, please try again", nil)
	errVerificationCodeInvalid = common.NewAPIError(common.APICodeVerificationCodeInvalid, "Verification code is incorrect or has expired", nil)
	errInvalidUserID           = common.NewAPIError(common.APICodeInvalidUserID, "invalid user ID", nil)
	errUserNotFound            = common.NewAPIError(common.APICodeUserNotFound, "User not found", nil)
)

// emailSendFailed keeps the mail server's own text in message; the console shows a generic line.
func emailSendFailed(err error) error {
	return common.NewAPIError(common.APICodeEmailSendFailed, err.Error(), nil)
}

// userLookupError reports a missing user with a code instead of the database's "record not found";
// other errors pass through.
func userLookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errUserNotFound
	}
	return err
}
