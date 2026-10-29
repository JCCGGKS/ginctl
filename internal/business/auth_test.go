package business

import (
	"context"
	"testing"

	"gin-scaffold/internal/config"
	"gin-scaffold/internal/schema"
	"gin-scaffold/internal/svc"
)

func newTestServiceContext() *svc.ServiceContext {
	cfg := &config.Config{}
	cfg.Auth.JWTSecret = "test-secret"
	cfg.Auth.AccessExpireSeconds = 900
	return svc.NewServiceContext(cfg)
}

func TestLoginSuccess(t *testing.T) {
	svcCtx := newTestServiceContext()

	resp, err := NewAuthBusiness(context.Background(), svcCtx).Login(&schema.LoginRequest{
		Username: "admin",
		Password: "admin123",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if resp.AccessToken == "" {
		t.Fatal("Login() returned empty token")
	}
	if resp.User.ID != 1 || resp.User.Username != "admin" {
		t.Fatalf("Login() user = %+v, want id=1 username=admin", resp.User)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	svcCtx := newTestServiceContext()

	if _, err := NewAuthBusiness(context.Background(), svcCtx).Login(&schema.LoginRequest{
		Username: "admin",
		Password: "wrong",
	}); err == nil {
		t.Fatal("Login() expected error for wrong password, got nil")
	}
}

func TestLoginUnknownUser(t *testing.T) {
	svcCtx := newTestServiceContext()

	if _, err := NewAuthBusiness(context.Background(), svcCtx).Login(&schema.LoginRequest{
		Username: "nobody",
		Password: "admin123",
	}); err == nil {
		t.Fatal("Login() expected error for unknown user, got nil")
	}
}
