package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"time"

	"gin-scaffold/common"
	"gin-scaffold/common/logger"

	"github.com/gin-gonic/gin"
)

const (
	headerRequestID    = "X-Request-Id"
	maxRequestIDLength = 64
)

// RequestID assigns a request id (incoming header or generated) and echoes it
// back. The unified response envelope embeds it for traceability.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(headerRequestID)
		if !validRequestID(id) {
			id = generateID()
		}
		common.SetRequestID(c, id)
		ctx := logger.WithRequestID(c.Request.Context(), id)
		c.Request = c.Request.WithContext(ctx)
		c.Header(headerRequestID, id)
		c.Next()
	}
}

// validRequestID accepts only short, log-safe identifiers supplied by the
// caller. Anything else is replaced with a server-generated id.
func validRequestID(id string) bool {
	if id == "" || len(id) > maxRequestIDLength {
		return false
	}
	for i := 0; i < len(id); i++ {
		ch := id[i]
		if (ch >= 'a' && ch <= 'z') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') ||
			ch == '-' || ch == '_' || ch == '.' {
			continue
		}
		return false
	}
	return true
}

func generateID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "t" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(b[:])
}
