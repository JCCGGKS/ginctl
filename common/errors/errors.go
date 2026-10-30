// Package errors defines the shared AppError type, business codes, and
// constructors. Domain-specific sentinel errors live in internal/errors.
package errors

import (
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

// NewAppError builds an AppError with an explicit status and business code.
func NewAppError(httpStatus, code int, msg string) *AppError {
	return &AppError{HTTPStatus: httpStatus, Code: code, Msg: msg}
}

func BadRequest(msg string) *AppError {
	return NewAppError(http.StatusBadRequest, CodeBadRequest, msg)
}

func Unauthorized(msg string) *AppError {
	return NewAppError(http.StatusUnauthorized, CodeUnauthorized, msg)
}

func NotFound(msg string) *AppError {
	return NewAppError(http.StatusNotFound, CodeNotFound, msg)
}

func MethodNotAllowed(msg string) *AppError {
	return NewAppError(http.StatusMethodNotAllowed, CodeMethodNotAllowed, msg)
}

func Timeout(msg string) *AppError {
	return NewAppError(http.StatusRequestTimeout, CodeTimeout, msg)
}

func Internal(msg string) *AppError {
	return NewAppError(http.StatusInternalServerError, CodeInternal, msg)
}
