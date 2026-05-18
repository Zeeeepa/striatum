package apply

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

func TestReceiptLookupRequiresSchema(t *testing.T) {
	service := Service{}
	_, err := service.ShowReceipt(context.Background(), rpc.Envelope{Params: map[string]any{}})
	if rpcErr, ok := err.(*rpc.Error); !ok || rpcErr.Code != "daemon_db_missing" {
		t.Fatalf("err = %#v, want daemon_db_missing", err)
	}
}

func TestRotateFallbackSigningKeyCreatesLoadable0600Ed25519Key(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon", "signing_key")
	t.Setenv(EnvSigningKeyPath, path)

	result, err := RotateFallbackSigningKey(context.Background())
	if err != nil {
		t.Fatalf("rotate signing key: %v", err)
	}
	if result["status"] != "rotated" || result["key_storage"] != "fallback_file" {
		t.Fatalf("unexpected rotation result: %#v", result)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat key: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("key mode = %o, want 0600", info.Mode().Perm())
	}
	key, err := LoadFallbackSigningKey()
	if err != nil {
		t.Fatalf("load key: %v", err)
	}
	if key.KeyID != result["signing_key_id"] {
		t.Fatalf("key id = %q, result = %q", key.KeyID, result["signing_key_id"])
	}
	if got := base64.StdEncoding.EncodeToString(key.PublicKey); got != result["public_key"] {
		t.Fatalf("public key mismatch")
	}
	status := FallbackSigningKeyStatus()
	if status["supported"] != true || status["key_loaded"] != true {
		t.Fatalf("unexpected fallback status: %#v", status)
	}
}

func TestRotateFallbackSigningKeyReportsPreviousKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "signing_key")
	t.Setenv(EnvSigningKeyPath, path)

	first, err := RotateFallbackSigningKey(context.Background())
	if err != nil {
		t.Fatalf("first rotate: %v", err)
	}
	second, err := RotateFallbackSigningKey(context.Background())
	if err != nil {
		t.Fatalf("second rotate: %v", err)
	}
	if second["signing_key_id"] == first["signing_key_id"] {
		t.Fatalf("rotation reused signing key id: %#v", second)
	}
	if second["rotated_from_signing_key_id"] != first["signing_key_id"] {
		t.Fatalf("previous key id = %v, want %v", second["rotated_from_signing_key_id"], first["signing_key_id"])
	}
	if second["rotated_from_public_key"] != first["public_key"] {
		t.Fatalf("previous public key = %v, want %v", second["rotated_from_public_key"], first["public_key"])
	}
}

func TestFallbackKeyLoadedRejectsInsecurePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "signing_key")
	t.Setenv(EnvSigningKeyPath, path)

	if _, err := RotateFallbackSigningKey(context.Background()); err != nil {
		t.Fatalf("rotate signing key: %v", err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("chmod key: %v", err)
	}
	if FallbackKeyLoaded() {
		t.Fatal("insecure fallback key reported loaded")
	}
	_, err := RotateFallbackSigningKey(context.Background())
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "signing_key_insecure" {
		t.Fatalf("err = %#v, want signing_key_insecure", err)
	}
}

func TestRotateFallbackSigningKeyBacksUpInvalidPrivateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "signing_key")
	t.Setenv(EnvSigningKeyPath, path)
	if err := os.WriteFile(path, []byte("legacy-placeholder\n"), 0o600); err != nil {
		t.Fatalf("write legacy key: %v", err)
	}

	result, err := RotateFallbackSigningKey(context.Background())
	if err != nil {
		t.Fatalf("rotate signing key: %v", err)
	}
	backupPath, ok := result["rotated_from_invalid_backup"].(string)
	if !ok || backupPath == "" {
		t.Fatalf("missing invalid backup path: %#v", result)
	}
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("stat invalid backup: %v", err)
	}
	body, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read invalid backup: %v", err)
	}
	if string(body) != "legacy-placeholder\n" {
		t.Fatalf("invalid backup content = %q", string(body))
	}
	if !FallbackKeyLoaded() {
		t.Fatal("new fallback key is not loadable")
	}
}
