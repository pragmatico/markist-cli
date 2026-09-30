package api

import "fmt"

// ErrorCode mirrors the ApiErrorCode union in src/lib/api-auth/errors.ts.
type ErrorCode string

const (
	ErrorCodeUnauthorized       ErrorCode = "unauthorized"
	ErrorCodeTokenExpired       ErrorCode = "token_expired"
	ErrorCodeOnboardingRequired ErrorCode = "onboarding_required"
	ErrorCodeForbidden          ErrorCode = "forbidden"
	ErrorCodeRateLimited        ErrorCode = "rate_limited"
	ErrorCodeValidationFailed   ErrorCode = "validation_failed"
	ErrorCodeUpgradeRequired    ErrorCode = "upgrade_required"
	ErrorCodeNotFound           ErrorCode = "not_found"
	ErrorCodeInternal           ErrorCode = "internal"
)

// ErrorBody mirrors the { error: { code, message } } envelope every
// /api/v1 route returns on failure, except the device-flow token/refresh
// endpoints (see TokenErrorCode in internal/auth), which use the RFC
// 8628/6749 shape instead.
type ErrorBody struct {
	Error struct {
		Code    ErrorCode `json:"code"`
		Message string    `json:"message"`
	} `json:"error"`
}

// Error is the typed error the client returns for a non-2xx /api/v1
// response. internal/cli's exit-code mapping (Task 10+) switches on Code.
type Error struct {
	StatusCode int
	Code       ErrorCode
	Message    string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s (HTTP %d)", e.Code, e.Message, e.StatusCode)
}
