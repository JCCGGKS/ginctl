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

	"gin-scaffold/infra/logger"
	"gin-scaffold/infra/requestid"

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
	CodeBadRequest       = 40000
	CodeUnauthorized     = 40100
	CodeNotFound         = 40400
	CodeMethodNotAllowed = 40500
	CodeTimeout          = 40800
	CodePayloadTooLarge  = 41300
	CodeTooManyRequests  = 42900
	CodeInternal         = 50000
	CodeUnavailable      = 50300
)

// gin.Context key used to carry per-request metadata.
const ctxRequestIDKey = "request_id"

// Body is the unified response envelope.
type Body struct {
	Code      int    `json:"code"`
	Msg       string `json:"msg"`
	Data      any    `json:"data"`
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
func NotFound(msg string) *AppError {
	return NewAppError(http.StatusNotFound, CodeNotFound, msg)
}
func MethodNotAllowed(msg string) *AppError {
	return NewAppError(http.StatusMethodNotAllowed, CodeMethodNotAllowed, msg)
}
func Timeout(msg string) *AppError {
	return NewAppError(http.StatusRequestTimeout, CodeTimeout, msg)
}
func PayloadTooLarge(msg string) *AppError {
	return NewAppError(http.StatusRequestEntityTooLarge, CodePayloadTooLarge, msg)
}
func TooManyRequests(msg string) *AppError {
	return NewAppError(http.StatusTooManyRequests, CodeTooManyRequests, msg)
}
func Internal(msg string) *AppError {
	return NewAppError(http.StatusInternalServerError, CodeInternal, msg)
}
func ServiceUnavailable(msg string) *AppError {
	return NewAppError(http.StatusServiceUnavailable, CodeUnavailable, msg)
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
		appErr = Internal("internal server error")
		fields := errorFields(c, appErr)
		fields = append(fields, zap.Error(err))
		logger.App().Error(c.Request.Context(), "unhandled error", fields...)
	} else if appErr.HTTPStatus >= http.StatusInternalServerError {
		fields := errorFields(c, appErr)
		fields = append(fields, zap.Error(err))
		logger.App().Error(c.Request.Context(), "request failed", fields...)
	}

	c.Abort()
	c.JSON(appErr.HTTPStatus, Body{
		Code:      appErr.Code,
		Msg:       appErr.Msg,
		RequestID: RequestID(c),
		Timestamp: time.Now().Unix(),
	})
}

func errorFields(c *gin.Context, appErr *AppError) []zap.Field {
	return []zap.Field{
		zap.String("method", c.Request.Method),
		zap.String("path", c.Request.URL.Path),
		zap.Int("status", appErr.HTTPStatus),
		zap.Int("code", appErr.Code),
	}
}

// --- per-request metadata helpers (used by middleware) ---

// SetRequestID stores the request id on the context.
func SetRequestID(c *gin.Context, id string) { c.Set(ctxRequestIDKey, id) }

// RequestID reads the request id, or "" if unset.
func RequestID(c *gin.Context) string {
	if id := getString(c, ctxRequestIDKey); id != "" {
		return id
	}
	if c.Request == nil {
		return ""
	}
	return requestid.FromContext(c.Request.Context())
}

func getString(c *gin.Context, key string) string {
	if v, ok := c.Get(key); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
