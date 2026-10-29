package token

import (
	"testing"
	"time"
)

func TestIssueAndParse(t *testing.T) {
	m := NewManager("test-secret", time.Minute)

	signed, expiresAt, err := m.Issue(7, "alice")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if signed == "" {
		t.Fatal("Issue() returned empty token")
	}
	if expiresAt.Before(time.Now()) {
		t.Fatalf("Issue() expiresAt = %v, want future", expiresAt)
	}

	var claims AuthClaims
	if err := m.ParseInto(signed, &claims); err != nil {
		t.Fatalf("ParseInto() error = %v", err)
	}
	if claims.UserID != 7 || claims.Username != "alice" {
		t.Fatalf("claims = %+v, want uid=7 username=alice", claims)
	}
}

func TestParseRejectsWrongSecret(t *testing.T) {
	issuer := NewManager("secret-a", time.Minute)
	verifier := NewManager("secret-b", time.Minute)

	signed, _, err := issuer.Issue(1, "bob")
	if err != nil {
		t.Fatal(err)
	}

	if err := verifier.ParseInto(signed, &AuthClaims{}); err == nil {
		t.Fatal("ParseInto() expected error with wrong secret, got nil")
	}
}

func TestIssueFailsWithoutSecret(t *testing.T) {
	m := NewManager("", time.Minute)
	if _, _, err := m.Issue(1, "alice"); err == nil {
		t.Fatal("Issue() expected error with empty secret, got nil")
	}
}

func TestExtractBearer(t *testing.T) {
	tests := []struct {
		header string
		want   string
	}{
		{"Bearer abc", "abc"},
		{"Bearer  abc ", "abc"},
		{"bearer abc", ""},
		{"Token abc", ""},
		{"", ""},
	}

	for _, tt := range tests {
		if got := ExtractBearer(tt.header); got != tt.want {
			t.Errorf("ExtractBearer(%q) = %q, want %q", tt.header, got, tt.want)
		}
	}
}
