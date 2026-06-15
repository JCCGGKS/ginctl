// Package response defines the unified HTTP response envelope and the typed
// application error model used across every layer.
//
// Contract (extracted from the reference project):
//   - every JSON endpoint returns the same shape: {code,msg,data,...}
//   - business code 0 == success, non-zero == failure
//   - handlers never hand-craft error payloads; they return an error and let
//     Fail map it to an HTTP status + body
package response

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	CodeOK = 0
	MsgOK  = "ok"
)

// gin.Context keys used to carry per-request metadata.
const (
	ctxRequestIDKey = "request_id"
	ctxBizCodeKey   = "biz_code"
	ctxBizMsgKey    = "biz_msg"
)

// Body is the unified response envelope.
type Body struct {
	Code      int    `json:"code"`
	Msg       string `json:"msg"`
	Data      any    `json:"data,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// AppError is a typed error carrying an HTTP status and a business code.
// Logic returns these; Fail turns them into responses.
type AppError struct {
	HTTPStatus int
	Code       int
	Msg        string
}

func (e *AppError) Error() string { return e.Msg }

func newAppError(status int, msg string) *AppError {
	// Code mirrors the HTTP status by default so clients can branch on one
	// field; override Code if you need finer-grained business codes.
	return &AppError{HTTPStatus: status, Code: status, Msg: msg}
}

func BadRequest(msg string) *AppError   { return newAppError(http.StatusBadRequest, msg) }
func Unauthorized(msg string) *AppError { return newAppError(http.StatusUnauthorized, msg) }
func Forbidden(msg string) *AppError    { return newAppError(http.StatusForbidden, msg) }
func NotFound(msg string) *AppError     { return newAppError(http.StatusNotFound, msg) }
func Conflict(msg string) *AppError     { return newAppError(http.StatusConflict, msg) }
func Internal(msg string) *AppError     { return newAppError(http.StatusInternalServerError, msg) }

// OK writes a success envelope with HTTP 200.
func OK(c *gin.Context, data any) {
	setOutcome(c, CodeOK, MsgOK)
	c.JSON(http.StatusOK, Body{
		Code:      CodeOK,
		Msg:       MsgOK,
		Data:      data,
		RequestID: RequestID(c),
		Timestamp: time.Now().Unix(),
	})
}

// Fail maps an error to an HTTP status + unified error envelope. A known
// *AppError keeps its status/code; anything else degrades to 500.
func Fail(c *gin.Context, err error) {
	var appErr *AppError
	if !errors.As(err, &appErr) {
		appErr = Internal(err.Error())
	}

	setOutcome(c, appErr.Code, appErr.Msg)
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

func setOutcome(c *gin.Context, code int, msg string) {
	c.Set(ctxBizCodeKey, code)
	c.Set(ctxBizMsgKey, msg)
}

// BizCode reports the business code written by OK/Fail (defaults to CodeOK).
// The operation-log middleware reads this instead of re-parsing the body.
func BizCode(c *gin.Context) int {
	if v, ok := c.Get(ctxBizCodeKey); ok {
		if code, ok := v.(int); ok {
			return code
		}
	}
	return CodeOK
}

// BizMsg reports the business message written by OK/Fail.
func BizMsg(c *gin.Context) string { return getString(c, ctxBizMsgKey) }

func getString(c *gin.Context, key string) string {
	if v, ok := c.Get(key); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
