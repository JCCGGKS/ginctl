// Package common holds cross-cutting building blocks shared by the HTTP and
// application layers: the unified response envelope, JWT handling, and
// pagination helpers. Error definitions live in common/errors; request-id
// context helpers live in common/logger.
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

	apperrors "gin-scaffold/common/errors"
	"gin-scaffold/common/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	CodeOK = 0
	MsgOK  = "ok"
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
	var appErr *apperrors.AppError
	if !stderrors.As(err, &appErr) {
		appErr = apperrors.Internal("internal server error")
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

func errorFields(c *gin.Context, appErr *apperrors.AppError) []zap.Field {
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
