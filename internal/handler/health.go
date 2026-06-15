// Package handler adapts HTTP to the logic layer: bind input, call logic,
// write a unified response. No business branching lives here.
package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Health is a liveness probe. It deliberately bypasses the unified envelope so
// load balancers can match a trivial body/status — the same pragmatism the
// reference project applies to its redirect endpoint.
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   time.Now().Unix(),
	})
}
