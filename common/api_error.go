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

	// Shared by many endpoints.
	APICodeInvalidParams           = "invalid_params"
	APICodeInvalidInput            = "invalid_input"
	APICodeInvalidEmail            = "invalid_email"
	APICodeSessionSaveFailed       = "session_save_failed"
	APICodeVerificationCodeInvalid = "verification_code_invalid"
	APICodeEmailSendFailed         = "email_send_failed"

	// Field validation; params carry the limit (min / max) from the validate tag.
	APICodeUsernameRequired    = "username_required"
	APICodeUsernameTooLong     = "username_too_long"
	APICodeUsernameHasAt       = "username_has_at"
	APICodePasswordRequired    = "password_required"
	APICodePasswordTooShort    = "password_too_short"
	APICodePasswordTooLong     = "password_too_long"
	APICodeDisplayNameTooLong  = "display_name_too_long"
	APICodeEmailTooLong        = "email_too_long"
	APICodeUsernameTaken       = "username_taken"
	APICodeEmailTaken          = "email_taken"
	APICodeInviteCodeRequired  = "invite_code_required"
	APICodeInviteCodeInvalid   = "invite_code_invalid"
	APICodeEmailNotRegistered  = "email_not_registered"
	APICodeEmailSharedAccounts = "email_shared_by_accounts"

	// Sign-in, sign-up and password reset.
	APICodePasswordLoginDisabled     = "password_login_disabled"
	APICodeLoginFailed               = "login_failed"
	APICodeEmailCodeLoginDisabled    = "email_code_login_disabled"
	APICodeRegisterDisabled          = "register_disabled"
	APICodePasswordRegisterDisabled  = "password_register_disabled"
	APICodeEmailVerificationRequired = "email_verification_required"
	APICodeEmailDomainNotAllowed     = "email_domain_not_allowed"
	APICodePasswordManagedExternally = "password_managed_externally"
	APICodeResetLinkInvalid          = "reset_link_invalid"
	APICodeTurnstileTokenMissing     = "turnstile_token_missing"
	APICodeTurnstileFailed           = "turnstile_failed"
	APICodeTurnstileUnavailable      = "turnstile_unavailable"

	// Session and permission checks on signed-in requests.
	APICodeNotSignedIn        = "not_signed_in"
	APICodeInvalidAccessToken = "invalid_access_token"
	APICodeInvalidSession     = "invalid_session"
	APICodeSessionExpired     = "session_expired"
	APICodeUserCheckFailed    = "user_check_failed"
	APICodeUserBanned         = "user_banned"
	APICodePermissionDenied   = "permission_denied"

	// Account security page.
	APICodeSelfPasswordUnavailable    = "self_password_unavailable"
	APICodeCurrentPasswordRequired    = "current_password_required"
	APICodeCurrentPasswordIncorrect   = "current_password_incorrect"
	APICodeCurrentAccountLoadFailed   = "current_account_load_failed"
	APICodeSessionCannotRevokeCurrent = "session_cannot_revoke_current"
	APICodeSessionNotFound            = "session_not_found"
	APICodeUnbindUnknownType          = "unbind_unknown_type"
	APICodeUnbindLastMethod           = "unbind_last_method"
	APICodeUnbindNoMethod             = "unbind_no_method"
	APICodeUnbindUnknownProvider      = "unbind_unknown_provider"

	// Admin user management and quota.
	APICodeInvalidUserID         = "invalid_user_id"
	APICodeUserNotFound          = "user_not_found"
	APICodeUserShadowAccount     = "user_shadow_account"
	APICodeUserRoleViewDenied    = "user_role_view_denied"
	APICodeUserRoleUpdateDenied  = "user_role_update_denied"
	APICodeUserRoleRaiseDenied   = "user_role_raise_denied"
	APICodeUserRoleDeleteDenied  = "user_role_delete_denied"
	APICodeUserRoleCreateDenied  = "user_role_create_denied"
	APICodeUserIDRequired        = "user_id_required"
	APICodeRootCannotDisable     = "root_cannot_disable"
	APICodeRootCannotDelete      = "root_cannot_delete"
	APICodeAdminPromoteRootOnly  = "admin_promote_root_only"
	APICodeRootRoleUnchangeable  = "root_role_unchangeable"
	APICodeTrustedStaffAdminOnly = "trusted_staff_admin_only"
	APICodeQuotaChangeZero       = "quota_change_zero"
	APICodeQuotaNegative         = "quota_negative"
	APICodeUserQuotaInsufficient = "user_quota_insufficient"

	// Organizations, their members, invitations, budgets, billing and API keys.
	APICodeOrgFeatureDisabled               = "org_feature_disabled"
	APICodeOrgInvalidID                     = "org_invalid_id"
	APICodeOrgNotFound                      = "org_not_found"
	APICodeOrgDisabled                      = "org_disabled"
	APICodeOrgNotMember                     = "org_not_member"
	APICodeOrgPermissionDenied              = "org_permission_denied"
	APICodeOrgUserNotMember                 = "org_user_not_member"
	APICodeOrgUserAlreadyMember             = "org_user_already_member"
	APICodeOrgAlreadyMember                 = "org_already_member"
	APICodeOrgMemberLimitReached            = "org_member_limit_reached"
	APICodeOrgLimitPerUser                  = "org_limit_per_user"
	APICodeOrgNameRequired                  = "org_name_required"
	APICodeOrgNameConfirmationMismatch      = "org_name_confirmation_mismatch"
	APICodeOrgCreateRequiresVerifiedContact = "org_create_requires_verified_contact"
	APICodeOrgStatusInvalid                 = "org_status_invalid"
	APICodeOrgDeleteBalanceBusy             = "org_delete_balance_busy"
	APICodeOrgRoleInvalid                   = "org_role_invalid"
	APICodeOrgMemberRoleHigherCreate        = "org_member_role_higher_create"
	APICodeOrgMemberRoleHigherModify        = "org_member_role_higher_modify"
	APICodeOrgMemberRoleSetHigher           = "org_member_role_set_higher"
	APICodeOrgMemberRoleHigherRemove        = "org_member_role_higher_remove"
	APICodeOrgCannotChangeOwnRole           = "org_cannot_change_own_role"
	APICodeOrgCannotRemoveSelf              = "org_cannot_remove_self"
	APICodeOrgOwnerRoleUnchangeable         = "org_owner_role_unchangeable"
	APICodeOrgOwnerCannotLeave              = "org_owner_cannot_leave"
	APICodeOrgTransferToSelf                = "org_transfer_to_self"
	APICodeOrgTransferTargetNotAdmin        = "org_transfer_target_not_admin"
	APICodeOrgTransferOwnerOnly             = "org_transfer_owner_only"
	APICodeOrgInvitationPendingExists       = "org_invitation_pending_exists"
	APICodeOrgInvitationMaxUsesNegative     = "org_invitation_max_uses_negative"
	APICodeExpiryInPast                     = "expiry_in_past"
	APICodeOrgInvitationNotFound            = "org_invitation_not_found"
	APICodeOrgInvitationNotForYou           = "org_invitation_not_for_you"
	APICodeOrgInvitationAccepted            = "org_invitation_accepted"
	APICodeOrgInvitationDeclined            = "org_invitation_declined"
	APICodeOrgInvitationRevoked             = "org_invitation_revoked"
	APICodeOrgInvitationExpired             = "org_invitation_expired"
	APICodeOrgInvitationUsedUp              = "org_invitation_used_up"
	APICodeOrgInvitationInvalid             = "org_invitation_invalid"
	APICodeOrgInvitationLinkNoDecline       = "org_invitation_link_no_decline"
	APICodeOrgBudgetInvalid                 = "org_budget_invalid"
	APICodeOrgBudgetPeriodInvalid           = "org_budget_period_invalid"
	APICodeOrgQuotaTransferDisabled         = "org_quota_transfer_disabled"
	APICodeOrgTransferAmountInvalid         = "org_transfer_amount_invalid"
	APICodeOrgTransferInsufficientBalance   = "org_transfer_insufficient_balance"
	APICodePaymentAmountTooSmall            = "payment_amount_too_small"
	APICodePaymentCreateFailed              = "payment_create_failed"
	APICodeOrderCreateFailed                = "order_create_failed"
	APICodeOrgUsageViewDenied               = "org_usage_view_denied"
	APICodeTokenNotFound                    = "token_not_found"
	APICodeTokenAccessDenied                = "token_access_denied"
	APICodeTokenNameTooLong                 = "token_name_too_long"
	APICodeTokenExpiredCannotEnable         = "token_expired_cannot_enable"
	APICodeTokenQuotaExhaustedCannotEnable  = "token_quota_exhausted_cannot_enable"

	// System settings.
	APICodeOptionIdpReadFailed               = "option_idp_read_failed"
	APICodeOptionExternalRequiresSiteIdp     = "option_external_requires_site_idp"
	APICodeOptionAccountSystemInvalid        = "option_account_system_invalid"
	APICodeOptionGithubOauthIncomplete       = "option_github_oauth_incomplete"
	APICodeOptionLinuxdoOauthIncomplete      = "option_linuxdo_oauth_incomplete"
	APICodeOptionLinuxdoTrustRequiresOauth   = "option_linuxdo_trust_requires_oauth"
	APICodeOptionLinuxdoDynamicRequiresLimit = "option_linuxdo_dynamic_requires_limit"
	APICodeOptionLinuxdoTrustNotNumber       = "option_linuxdo_trust_not_number"
	APICodeOptionLinuxdoTrustOutOfRange      = "option_linuxdo_trust_out_of_range"
	APICodeOptionSmtpEncryptionInvalid       = "option_smtp_encryption_invalid"
	APICodeOptionEmailDomainsRequired        = "option_email_domains_required"
	APICodeOptionWechatIncomplete            = "option_wechat_incomplete"
	APICodeOptionTurnstileIncomplete         = "option_turnstile_incomplete"
	APICodeOptionQuotaNotInteger             = "option_quota_not_integer"
	APICodeOptionTopupBonusNotNumber         = "option_topup_bonus_not_number"
	APICodeOptionTopupBonusPercentRange      = "option_topup_bonus_percent_range"
	APICodeOptionTopupBonusFixedNegative     = "option_topup_bonus_fixed_negative"

	// Invite codes, redemption codes and bulk actions.
	APICodeInviteCodeHasSpaces        = "invite_code_has_spaces"
	APICodeInviteCodeLength           = "invite_code_length"
	APICodeInviteCodeCharset          = "invite_code_charset"
	APICodeInviteCodeLoadFailed       = "invite_code_load_failed"
	APICodeBatchTooLarge              = "batch_too_large"
	APICodeStartAfterExpiry           = "start_after_expiry"
	APICodeInviteCodeExists           = "invite_code_exists"
	APICodeInviteCodeCreateFailed     = "invite_code_create_failed"
	APICodeInviteCodeMaxUsesNegative  = "invite_code_max_uses_negative"
	APICodeInviteCodeMaxUsesBelowUsed = "invite_code_max_uses_below_used"
	APICodeStatusInvalid              = "status_invalid"
	APICodeInviteCodeUsedUp           = "invite_code_used_up"
	APICodeDeleteSelectionRequired    = "delete_selection_required"
	APICodeBulkDeleteFailed           = "bulk_delete_failed"
	APICodeInviteCodeRegisterDisabled = "invite_code_register_disabled"
	APICodeSystemError                = "system_error"
	APICodeRedemptionNameLength       = "redemption_name_length"
	APICodeRedemptionCountInvalid     = "redemption_count_invalid"
	APICodeRedemptionQuotaInvalid     = "redemption_quota_invalid"
	APICodeRedemptionPurposeInvalid   = "redemption_purpose_invalid"
	APICodeRedemptionCodeRequired     = "redemption_code_required"
	APICodeRedemptionCodeInvalid      = "redemption_code_invalid"
	APICodeRedemptionCodeUsed         = "redemption_code_used"
	APICodeRedemptionCodeExpired      = "redemption_code_expired"
	APICodeRedemptionCodeOrgOnly      = "redemption_code_org_only"
	APICodeRedemptionCodePersonalOnly = "redemption_code_personal_only"
	APICodeInviteCodeSet              = "invite_code_set"

	// API keys, user groups and the model catalog.
	APICodeTokenChatCreateFailed        = "token_chat_create_failed"
	APICodeUserGroupLoadFailed          = "user_group_load_failed"
	APICodeUserGroupInvalid             = "user_group_invalid"
	APICodeUserGroupNotAllowed          = "user_group_not_allowed"
	APICodeTokenHeartbeatRange          = "token_heartbeat_range"
	APICodeTokenQuotaResetPeriodInvalid = "token_quota_reset_period_invalid"
	APICodeTokenQuotaResetLimitInvalid  = "token_quota_reset_limit_invalid"
	APICodeModelModeInvalid             = "model_mode_invalid"
	APICodeModelCapabilitiesInvalid     = "model_capabilities_invalid"
	APICodeModelEndpointsInvalid        = "model_endpoints_invalid"
	APICodeModelIdentifierExists        = "model_identifier_exists"
	APICodeModelReferencedByAliases     = "model_referenced_by_aliases"
	APICodeModelAliasSelf               = "model_alias_self"
	APICodeModelAliasTargetMissing      = "model_alias_target_missing"
	APICodeModelAliasTargetIsAlias      = "model_alias_target_is_alias"

	// Orders, payment methods and invoices.
	APICodePaymentNotFound      = "payment_not_found"
	APICodeInvalidTimeFormat    = "invalid_time_format"
	APICodeInvoiceMonthNotPast  = "invoice_month_not_past"
	APICodeInvoiceMonthExists   = "invoice_month_exists"
	APICodeInvoiceTaskSubmitted = "invoice_task_submitted"
	APICodePaymentAdded         = "payment_added"

	// Third-party sign-in. params.provider is the provider name.
	APICodeOauthStateInvalid        = "oauth_state_invalid"
	APICodeOauthProviderDisabled    = "oauth_provider_disabled"
	APICodeOauthProviderUnreachable = "oauth_provider_unreachable"
	APICodeOauthAccountLinked       = "oauth_account_linked"
	APICodeOauthInvalidResponse     = "oauth_invalid_response"
	APICodeOauthCodeInvalid         = "oauth_code_invalid"
	APICodeLinuxdoTrustTooLow       = "linuxdo_trust_too_low"
	APICodeLinuxdoAlreadyBound      = "linuxdo_already_bound"
	APICodeOidcLoginFailed          = "oidc_login_failed"
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
