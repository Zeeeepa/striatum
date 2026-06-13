package db

import (
	"strings"
	"testing"
)

func TestValidateDSN(t *testing.T) {
	if err := ValidateDSN(""); err == nil {
		t.Fatal("empty DSN should be a config error")
	}
	if err := ValidateDSN("postgres://u@/db?host=/var/run/postgresql"); err != nil {
		t.Fatalf("valid DSN rejected: %v", err)
	}
	// Invalid percent-encoding in the userinfo: url.Parse (inside pgx) rejects it,
	// so this is a deterministic config error, not a connectivity failure.
	err := ValidateDSN("postgres://user:p%ssword@localhost/db")
	if err == nil {
		t.Fatal("malformed DSN should be a config error")
	}
	if strings.Contains(err.Error(), "p%ssword") || strings.Contains(err.Error(), "pssword") {
		t.Fatalf("DSN error must redact credentials: %v", err)
	}
}
