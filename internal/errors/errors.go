// Package errors defines application transport errors, business codes, and
// domain-specific sentinel errors.
package errors

import (
	stderrors "errors"
	"net/http"
)

// Business codes are independent of HTTP status so clients can branch on one
// stable namespace even as transport details change.
const (
	CodeBadRequest       = 40000
	CodeUnauthorized     = 40100
	CodeForbidden        = 40300
	CodeNotFound         = 40400
	CodeMethodNotAllowed = 40500
	CodeTimeout          = 40800
	CodeTooManyRequests  = 42900
	CodeInternal         = 50000
)

// AppError is a typed error carrying an HTTP status and an independent business
// code. Logic returns these; common.Fail turns them into responses.
type AppError struct {
	HTTPStatus int
	Code       int
	Msg        string
}

func (e *AppError) Error() string { return e.Msg }

// HTTPStatusCode implements common.HTTPError.
func (e *AppError) HTTPStatusCode() int { return e.HTTPStatus }

// BusinessCode implements common.HTTPError.
func (e *AppError) BusinessCode() int { return e.Code }

// PublicMessage implements common.HTTPError.
func (e *AppError) PublicMessage() string { return e.Msg }

// NewAppError builds an AppError with an explicit status and business code.
func NewAppError(httpStatus, code int, msg string) *AppError {
	return &AppError{HTTPStatus: httpStatus, Code: code, Msg: msg}
}

func BadRequest(msg string) error {
	return NewAppError(http.StatusBadRequest, CodeBadRequest, msg)
}

func Unauthorized(msg string) error {
	return NewAppError(http.StatusUnauthorized, CodeUnauthorized, msg)
}

func Forbidden(msg string) error {
	return NewAppError(http.StatusForbidden, CodeForbidden, msg)
}

func NotFound(msg string) error {
	return NewAppError(http.StatusNotFound, CodeNotFound, msg)
}

func MethodNotAllowed(msg string) error {
	return NewAppError(http.StatusMethodNotAllowed, CodeMethodNotAllowed, msg)
}

func Timeout(msg string) error {
	return NewAppError(http.StatusRequestTimeout, CodeTimeout, msg)
}

func TooManyRequests(msg string) error {
	return NewAppError(http.StatusTooManyRequests, CodeTooManyRequests, msg)
}

func Internal(msg string) error {
	return NewAppError(http.StatusInternalServerError, CodeInternal, msg)
}

var (
	// ErrUserNotFound is returned when a user lookup has no match.
	ErrUserNotFound = stderrors.New("user not found")

	// ErrInvalidCredentials is returned for both unknown users and wrong
	// passwords. It is already a public AppError, so every transport maps it to
	// the same 401 response and callers cannot distinguish account existence.
	ErrInvalidCredentials = NewAppError(http.StatusUnauthorized, CodeUnauthorized, "username or password is invalid")

	// ErrInvalidRefreshToken is the single public response for every refresh
	// failure, so callers cannot distinguish expired, malformed, or unknown-user
	// tokens.
	ErrInvalidRefreshToken = NewAppError(http.StatusUnauthorized, CodeUnauthorized, "refresh token is invalid")

	// ErrCacheMiss reports that the requested cache key is not present.
	ErrCacheMiss = stderrors.New("cache miss")

	// ErrCacheUnavailable reports that the cache backend is unavailable.
	ErrCacheUnavailable = stderrors.New("cache unavailable")
)
