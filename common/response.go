// Package common holds cross-cutting building blocks shared by the HTTP and
// application layers: the unified response envelope, JWT handling, and
// pagination helpers. Application and domain errors live in each service's
// internal/errors package, and request-id context helpers live in
// common/logger.
//
// Contract:
//   - every JSON endpoint returns the same shape: {code,msg,data,...}
//   - business code 0 == success, non-zero == failure
//   - handlers never hand-craft error payloads; they return an error and let
//     Fail map it to an HTTP status + body
package common

import (
	stderrors "errors"
	"net/http"
	"time"

	"gin-scaffold/common/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	CodeOK = 0
	MsgOK  = "ok"

	fallbackCodeInternal = 50000
	fallbackMsgInternal  = "internal server error"
)

// HTTPError is implemented by application errors that carry their own HTTP
// status and business code. Keeping this as an interface lets common stay
// independent from each service's internal/errors package.
type HTTPError interface {
	error
	HTTPStatusCode() int
	BusinessCode() int
	PublicMessage() string
}

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
// HTTPError keeps its status/code; anything else is logged and degraded to a
// generic 500 so internal details are not leaked to the client.
func Fail(c *gin.Context, err error) {
	var httpErr HTTPError
	if !stderrors.As(err, &httpErr) {
		fields := errorFields(c, http.StatusInternalServerError, fallbackCodeInternal)
		fields = append(fields, zap.Error(err))
		logger.App().Error(c.Request.Context(), "unhandled error", fields...)
		writeError(c, http.StatusInternalServerError, fallbackCodeInternal, fallbackMsgInternal)
		return
	}

	status := httpErr.HTTPStatusCode()
	code := httpErr.BusinessCode()
	if status >= http.StatusInternalServerError {
		fields := errorFields(c, status, code)
		fields = append(fields, zap.Error(err))
		logger.App().Error(c.Request.Context(), "request failed", fields...)
	}

	writeError(c, status, code, httpErr.PublicMessage())
}

func writeError(c *gin.Context, status, code int, msg string) {
	c.Abort()
	c.JSON(status, Body{
		Code:      code,
		Msg:       msg,
		RequestID: RequestID(c),
		Timestamp: time.Now().Unix(),
	})
}

func errorFields(c *gin.Context, status, code int) []zap.Field {
	return []zap.Field{
		zap.String("method", c.Request.Method),
		zap.String("path", c.Request.URL.Path),
		zap.Int("status", status),
		zap.Int("code", code),
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
	return logger.RequestIDFromContext(c.Request.Context())
}

func getString(c *gin.Context, key string) string {
	if v, ok := c.Get(key); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
