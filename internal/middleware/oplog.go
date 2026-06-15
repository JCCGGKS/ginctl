package middleware

import (
	"context"
	"log/slog"

	"gin-scaffold/pkg/response"

	"github.com/gin-gonic/gin"
)

// OperationRecord is one audit entry.
type OperationRecord struct {
	UserID    uint64
	Action    string
	Result    string
	Detail    string
	RequestID string
}

const (
	resultSuccess = "success"
	resultFailed  = "failed"
)

// OperationSink persists audit records. The reference project writes them to
// MySQL via a DAO; here the default sink logs them. Swap in a DB-backed sink
// by implementing this interface and passing it to OperationLog.
type OperationSink interface {
	Save(ctx context.Context, rec OperationRecord) error
}

// OperationLog records an audit entry for the wrapped route.
//
// Unlike the reference oplog (which buffers and re-parses the response body),
// this version reads the business outcome the handler already stored on the
// context via response.OK/Fail — no body capture, far less fragile.
func OperationLog(action string, sink OperationSink) gin.HandlerFunc {
	if sink == nil {
		sink = slogSink{}
	}
	return func(c *gin.Context) {
		c.Next()

		result := resultSuccess
		if c.Writer.Status() >= 400 || response.BizCode(c) != response.CodeOK {
			result = resultFailed
		}

		var userID uint64
		if claims, ok := CurrentClaims(c); ok && claims != nil {
			userID = claims.UserID
		}

		rec := OperationRecord{
			UserID:    userID,
			Action:    action,
			Result:    result,
			Detail:    response.BizMsg(c),
			RequestID: response.RequestID(c),
		}
		if err := sink.Save(c.Request.Context(), rec); err != nil {
			slog.Error("save operation log failed", "error", err, "action", action)
		}
	}
}

// slogSink is the default OperationSink: it logs the audit entry.
type slogSink struct{}

func (slogSink) Save(_ context.Context, rec OperationRecord) error {
	slog.Info("operation_log",
		"user_id", rec.UserID,
		"action", rec.Action,
		"result", rec.Result,
		"detail", rec.Detail,
		"request_id", rec.RequestID,
	)
	return nil
}
