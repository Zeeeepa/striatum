package apply

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

const signingKeySchemaVersion = 1
const signingKeyAlgorithm = "ed25519"

var (
	ErrSigningKeyMissing  = errors.New("signing key missing")
	ErrSigningKeyInvalid  = errors.New("signing key invalid")
	ErrSigningKeyInsecure = errors.New("signing key permissions are insecure")
)

type SigningKey struct {
	KeyID      string
	Algorithm  string
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
	CreatedAt  time.Time
	Path       string
}

type signingKeyFile struct {
	SchemaVersion      int     `json:"schema_version"`
	Algorithm          string  `json:"algorithm"`
	KeyID              string  `json:"key_id"`
	PublicKey          string  `json:"public_key"`
	PrivateKey         string  `json:"private_key"`
	CreatedAt          string  `json:"created_at"`
	RotatedFromKeyID   *string `json:"rotated_from_key_id,omitempty"`
	RotatedFromPublic  *string `json:"rotated_from_public_key,omitempty"`
	PrivateKeyEncoding string  `json:"private_key_encoding"`
}

func LoadFallbackSigningKey() (SigningKey, error) {
	return loadFallbackSigningKey(FallbackKeyPath())
}

func FallbackSigningKeyStatus() map[string]any {
	key, err := LoadFallbackSigningKey()
	if err != nil {
		return map[string]any{
			"supported":      false,
			"key_loaded":     false,
			"public_key":     nil,
			"signing_key_id": nil,
		}
	}
	return map[string]any{
		"supported":      true,
		"key_loaded":     true,
		"public_key":     base64.StdEncoding.EncodeToString(key.PublicKey),
		"signing_key_id": key.KeyID,
		"algorithm":      key.Algorithm,
		"key_storage":    "fallback_file",
	}
}

func RotateFallbackSigningKey(ctx context.Context) (map[string]any, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	path := FallbackKeyPath()
	now := time.Now().UTC()
	var previousID any
	var previousPublic any
	var invalidBackup any
	if previous, err := LoadFallbackSigningKey(); err == nil {
		previousID = previous.KeyID
		previousPublic = base64.StdEncoding.EncodeToString(previous.PublicKey)
	} else if errors.Is(err, ErrSigningKeyInvalid) {
		backupPath, backupErr := backupInvalidFallbackSigningKey(path, now)
		if backupErr != nil {
			return nil, backupErr
		}
		invalidBackup = backupPath
	} else if !errors.Is(err, ErrSigningKeyMissing) {
		return nil, signingKeyRPCError(err)
	}

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	keyID := signingKeyID(publicKey)
	record := signingKeyFile{
		SchemaVersion:      signingKeySchemaVersion,
		Algorithm:          signingKeyAlgorithm,
		KeyID:              keyID,
		PublicKey:          base64.StdEncoding.EncodeToString(publicKey),
		PrivateKey:         base64.StdEncoding.EncodeToString(privateKey),
		CreatedAt:          now.Format(time.RFC3339Nano),
		PrivateKeyEncoding: "ed25519-private-key",
	}
	if previousIDString, ok := previousID.(string); ok && previousIDString != "" {
		record.RotatedFromKeyID = &previousIDString
	}
	if previousPublicString, ok := previousPublic.(string); ok && previousPublicString != "" {
		record.RotatedFromPublic = &previousPublicString
	}
	if err := writeFallbackSigningKey(path, record); err != nil {
		return nil, err
	}
	return map[string]any{
		"status":                       "rotated",
		"key_kind":                     "sealed_apply_signing_key",
		"algorithm":                    signingKeyAlgorithm,
		"key_storage":                  "fallback_file",
		"key_path":                     path,
		"signing_key_id":               keyID,
		"public_key":                   record.PublicKey,
		"rotated_from_signing_key_id":  previousID,
		"rotated_from_public_key":      previousPublic,
		"rotated_from_invalid_backup":  invalidBackup,
		"rotated_at":                   record.CreatedAt,
		"key_loaded":                   true,
		"sealed_apply_supported_after": true,
	}, nil
}

func FallbackKeyPath() string {
	if override := os.Getenv(EnvSigningKeyPath); override != "" {
		return filepath.Clean(expandHomePath(override))
	}
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		stateHome = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateHome, "striatum", "daemon", "signing_key")
}

func loadFallbackSigningKey(path string) (SigningKey, error) {
	if path == "" {
		return SigningKey{}, fmt.Errorf("%w: fallback signing key path is empty", ErrSigningKeyMissing)
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return SigningKey{}, ErrSigningKeyMissing
	}
	if err != nil {
		return SigningKey{}, err
	}
	if info.IsDir() {
		return SigningKey{}, fmt.Errorf("%w: fallback signing key path is a directory", ErrSigningKeyInvalid)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return SigningKey{}, fmt.Errorf("%w: fallback signing key must not be readable by group or other", ErrSigningKeyInsecure)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return SigningKey{}, err
	}
	var record signingKeyFile
	if err := json.Unmarshal(body, &record); err != nil {
		return SigningKey{}, fmt.Errorf("%w: %v", ErrSigningKeyInvalid, err)
	}
	publicKey, privateKey, createdAt, err := validateSigningKeyFile(record)
	if err != nil {
		return SigningKey{}, err
	}
	return SigningKey{
		KeyID:      record.KeyID,
		Algorithm:  record.Algorithm,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		CreatedAt:  createdAt,
		Path:       path,
	}, nil
}

func validateSigningKeyFile(record signingKeyFile) (ed25519.PublicKey, ed25519.PrivateKey, time.Time, error) {
	if record.SchemaVersion != signingKeySchemaVersion {
		return nil, nil, time.Time{}, fmt.Errorf("%w: unsupported schema_version", ErrSigningKeyInvalid)
	}
	if record.Algorithm != signingKeyAlgorithm {
		return nil, nil, time.Time{}, fmt.Errorf("%w: unsupported algorithm", ErrSigningKeyInvalid)
	}
	publicKey, err := base64.StdEncoding.DecodeString(record.PublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return nil, nil, time.Time{}, fmt.Errorf("%w: malformed public_key", ErrSigningKeyInvalid)
	}
	privateKey, err := base64.StdEncoding.DecodeString(record.PrivateKey)
	if err != nil {
		return nil, nil, time.Time{}, fmt.Errorf("%w: malformed private_key", ErrSigningKeyInvalid)
	}
	if len(privateKey) == ed25519.SeedSize {
		privateKey = ed25519.NewKeyFromSeed(privateKey)
	}
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, nil, time.Time{}, fmt.Errorf("%w: malformed private_key", ErrSigningKeyInvalid)
	}
	derived, ok := ed25519.PrivateKey(privateKey).Public().(ed25519.PublicKey)
	if !ok || !bytes.Equal(derived, publicKey) {
		return nil, nil, time.Time{}, fmt.Errorf("%w: public/private key mismatch", ErrSigningKeyInvalid)
	}
	if record.KeyID != signingKeyID(publicKey) {
		return nil, nil, time.Time{}, fmt.Errorf("%w: key_id does not match public key", ErrSigningKeyInvalid)
	}
	createdAt, err := time.Parse(time.RFC3339Nano, record.CreatedAt)
	if err != nil {
		return nil, nil, time.Time{}, fmt.Errorf("%w: malformed created_at", ErrSigningKeyInvalid)
	}
	return ed25519.PublicKey(publicKey), ed25519.PrivateKey(privateKey), createdAt, nil
}

func writeFallbackSigningKey(path string, record signingKeyFile) error {
	if path == "" {
		return fmt.Errorf("%w: fallback signing key path is empty", ErrSigningKeyMissing)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	body, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	tmp, err := os.CreateTemp(dir, ".signing_key.*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func backupInvalidFallbackSigningKey(path string, now time.Time) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", signingKeyRPCError(fmt.Errorf("%w: fallback signing key path is not a regular file", ErrSigningKeyInvalid))
	}
	backupPath := path + ".invalid." + now.Format("20060102T150405.000000000Z")
	if err := os.Rename(path, backupPath); err != nil {
		return "", err
	}
	if err := os.Chmod(backupPath, 0o600); err != nil {
		return "", err
	}
	return backupPath, nil
}

func signingKeyID(publicKey ed25519.PublicKey) string {
	sum := sha256.Sum256(publicKey)
	return "ed25519:" + hex.EncodeToString(sum[:16])
}

func signingKeyRPCError(err error) error {
	details := map[string]any{
		"operation":         "daemon.key.rotate",
		"key_kind":          "sealed_apply_signing_key",
		"key_storage":       "fallback_file",
		"python_dependency": false,
		"sqlite_dependency": false,
	}
	switch {
	case errors.Is(err, ErrSigningKeyInsecure):
		return rpc.NewError("signing_key_insecure", err.Error(), details)
	case errors.Is(err, ErrSigningKeyInvalid):
		return rpc.NewError("signing_key_invalid", err.Error(), details)
	default:
		return err
	}
}

func expandHomePath(value string) string {
	if value == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return value
		}
		return home
	}
	if strings.HasPrefix(value, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return value
		}
		return filepath.Join(home, strings.TrimPrefix(value, "~/"))
	}
	return value
}
