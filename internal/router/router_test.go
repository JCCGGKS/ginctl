package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gin-scaffold/internal/config"
	"gin-scaffold/internal/svc"
)

func newTestServiceContext() *svc.ServiceContext {
	cfg := &config.Config{}
	cfg.Server.Mode = "test"
	cfg.Auth.JWTSecret = "test-secret"
	cfg.Auth.AccessExpireSeconds = 900
	return svc.NewServiceContext(cfg)
}

func TestHealthz(t *testing.T) {
	engine := New(newTestServiceContext())

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestLoginAndMeFlow(t *testing.T) {
	engine := New(newTestServiceContext())

	loginBody := `{"username":"admin","password":"admin123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/login", strings.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal login response: %v", err)
	}
	if resp.Code != 0 || resp.Data.AccessToken == "" {
		t.Fatalf("login response = %+v, want code=0 with token", resp)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+resp.Data.AccessToken)
	meW := httptest.NewRecorder()
	engine.ServeHTTP(meW, meReq)

	if meW.Code != http.StatusOK {
		t.Fatalf("me status = %d, want %d; body=%s", meW.Code, http.StatusOK, meW.Body.String())
	}
	if !strings.Contains(meW.Body.String(), `"username":"admin"`) {
		t.Fatalf("me body = %s, want username admin", meW.Body.String())
	}
}

func TestMeRequiresAuth(t *testing.T) {
	engine := New(newTestServiceContext())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusUnauthorized, w.Body.String())
	}
}
