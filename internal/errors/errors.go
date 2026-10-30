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
	CodeNotFound         = 40400
	CodeMethodNotAllowed = 40500
	CodeTimeout          = 40800
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

func NotFound(msg string) error {
	return NewAppError(http.StatusNotFound, CodeNotFound, msg)
}

func MethodNotAllowed(msg string) error {
	return NewAppError(http.StatusMethodNotAllowed, CodeMethodNotAllowed, msg)
}

func Timeout(msg string) error {
	return NewAppError(http.StatusRequestTimeout, CodeTimeout, msg)
}

func Internal(msg string) error {
	return NewAppError(http.StatusInternalServerError, CodeInternal, msg)
}

var (
	// ErrUserNotFound is returned when a user lookup has no match.
	ErrUserNotFound = stderrors.New("user not found")

	// ErrInvalidCredentials is returned for both unknown users and wrong
	// passwords. Handlers map it to a single public 401 response so callers
	// cannot distinguish account existence.
	ErrInvalidCredentials = stderrors.New("invalid credentials")

	// ErrCacheMiss reports that the requested cache key is not present.
	ErrCacheMiss = stderrors.New("cache miss")

	// ErrCacheUnavailable reports that the cache backend is unavailable.
	ErrCacheUnavailable = stderrors.New("cache unavailable")
)
