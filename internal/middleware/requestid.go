package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"gin-scaffold/infra/requestid"
	"gin-scaffold/infra/response"

	"github.com/gin-gonic/gin"
)

const headerRequestID = "X-Request-Id"

// RequestID assigns a request id (incoming header or generated) and echoes it
// back. The unified response envelope embeds it for traceability.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(headerRequestID)
		if id == "" {
			id = generateID()
		}
		response.SetRequestID(c, id)
		ctx := requestid.WithContext(c.Request.Context(), id)
		c.Request = c.Request.WithContext(ctx)
		c.Header(headerRequestID, id)
		c.Next()
	}
}

func generateID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b[:])
}
