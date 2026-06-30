package model

import "errors"

// Common errors
var (
	ErrDatabase = errors.New("database error")
)

// User auth errors
var (
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrUserEmptyCredentials = errors.New("empty credentials")
	ErrUserDisabled         = errors.New("user disabled")
	ErrUserNotFound         = errors.New("user not found")
)

// Token auth errors
var (
	ErrTokenNotProvided = errors.New("token not provided")
	ErrTokenInvalid     = errors.New("token invalid")
)

// Redemption errors
var (
	ErrRedeemFailed          = errors.New("redeem.failed")
	ErrRedemptionInvalid     = errors.New("redemption.invalid")
	ErrRedemptionUsed        = errors.New("redemption.used")
	ErrRedemptionExpired     = errors.New("redemption.expired")
	ErrRedemptionNotProvided = errors.New("redemption.not_provided")
	ErrRedemptionExhausted   = errors.New("redemption.exhausted")
)

// 2FA errors
var ErrTwoFANotEnabled = errors.New("2fa not enabled")
