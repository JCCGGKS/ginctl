// Package response defines the unified HTTP response envelope and the typed
// application error model used across every layer.
//
// Contract:
//   - every JSON endpoint returns the same shape: {code,msg,data,...}
//   - business code 0 == success, non-zero == failure
//   - handlers never hand-craft error payloads; they return an error and let
//     Fail map it to an HTTP status + body
package response

import (
	"errors"
	"net/http"
	"time"

	"gin-scaffold/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	CodeOK = 0
	MsgOK  = "ok"
)

// Business codes are independent of HTTP status so clients can branch on one
// stable namespace even as transport details change. Add more codes here as
// the API grows.
const (
	CodeBadRequest   = 40000
	CodeUnauthorized = 40100
	CodeInternal     = 50000
)

// gin.Context key used to carry per-request metadata.
const ctxRequestIDKey = "request_id"

// Body is the unified response envelope.
type Body struct {
	Code      int    `json:"code"`
	Msg       string `json:"msg"`
	Data      any    `json:"data,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// AppError is a typed error carrying an HTTP status and an independent business
// code. Logic returns these; Fail turns them into responses.
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

func BadRequest(msg string) *AppError { return NewAppError(http.StatusBadRequest, CodeBadRequest, msg) }
func Unauthorized(msg string) *AppError {
	return NewAppError(http.StatusUnauthorized, CodeUnauthorized, msg)
}
func Internal(msg string) *AppError {
	return NewAppError(http.StatusInternalServerError, CodeInternal, msg)
}

// OK writes a success envelope with HTTP 200.
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{
		Code:      CodeOK,
		Msg:       MsgOK,
		Data:      data,
		RequestID: RequestID(c),
		Timestamp: time.Now().Unix(),
	})
}

// Fail maps an error to an HTTP status + unified error envelope. A known
// *AppError keeps its status/code; anything else is logged and degraded to a
// generic 500 so internal details are not leaked to the client.
func Fail(c *gin.Context, err error) {
	var appErr *AppError
	if !errors.As(err, &appErr) {
		logger.Error("unhandled error", zap.Error(err))
		appErr = Internal("internal server error")
	}

	c.JSON(appErr.HTTPStatus, Body{
		Code:      appErr.Code,
		Msg:       appErr.Msg,
		RequestID: RequestID(c),
		Timestamp: time.Now().Unix(),
	})
}

// --- per-request metadata helpers (used by middleware) ---

// SetRequestID stores the request id on the context.
func SetRequestID(c *gin.Context, id string) { c.Set(ctxRequestIDKey, id) }

// RequestID reads the request id, or "" if unset.
func RequestID(c *gin.Context) string { return getString(c, ctxRequestIDKey) }

func getString(c *gin.Context, key string) string {
	if v, ok := c.Get(key); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
