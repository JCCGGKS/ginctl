package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gin-scaffold/infra/token"

	"github.com/gin-gonic/gin"
)

func newAuthTestRouter(tm *token.Manager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Auth(tm))
	r.GET("/protected", func(c *gin.Context) {
		claims := CurrentClaims(c)
		if claims == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "missing claims"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"uid": claims.UserID})
	})
	return r
}

func TestAuthRequiresBearer(t *testing.T) {
	r := newAuthTestRouter(token.NewManager("secret", time.Minute))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthRejectsInvalidToken(t *testing.T) {
	r := newAuthTestRouter(token.NewManager("secret", time.Minute))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthSetsClaims(t *testing.T) {
	tm := token.NewManager("secret", time.Minute)
	signed, _, err := tm.Issue(7, "alice")
	if err != nil {
		t.Fatal(err)
	}

	r := newAuthTestRouter(tm)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if body := w.Body.String(); body != `{"uid":7}` {
		t.Fatalf("body = %s, want {\"uid\":7}", body)
	}
}
