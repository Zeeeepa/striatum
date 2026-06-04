package db

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

// Bootstrap postures (RFC 0110 §9). Reported by doctor; never the secret.
const (
	PostureSchemaAbsent   = "schema_absent"
	PostureSingleRoleSkip = "rotation_skipped_single_role"
	PostureRotated        = "rotated"
)

// BootstrapConfig describes the L0 credential bootstrap inputs (RFC 0110 §9).
type BootstrapConfig struct {
	// RuntimeURL is the daemon's runtime pool DSN.
	RuntimeURL string
	// OwnerURL is the owner-bootstrap DSN (STRIATUM_OWNER_DB_URL). Empty means
	// single-role: the runtime connection is itself the owner (PEER), and
	// password rotation is skipped.
	OwnerURL string
	// RuntimeRole is the role whose password is rotated and whose authority
	// secret is registered (registry label / rotator-probe scope).
	RuntimeRole string
	// InstanceID identifies this daemon process in the registry.
	InstanceID string
	// DaemonVersion tags the bootstrap connection.
	DaemonVersion string
}

// BootstrapResult is the outcome of BootstrapAuthority.
type BootstrapResult struct {
	// Secret is the per-instance daemon-authority secret to install via
	// SetAuthorityRuntime; empty when inert (no authority schema).
	Secret string
	// NewRuntimeURL is set only when the runtime password was rotated; the
	// caller must reconnect the runtime pool with it.
	NewRuntimeURL string
	// Posture is one of the Posture* constants.
	Posture string
	// RotatorCollision is true when another instance recently rotated the same
	// role (role-scoped probe, §9.4).
	RotatorCollision bool
	// Registered is true when an authority secret was registered.
	Registered bool
}

// BootstrapAuthority runs the RFC 0110 L0 credential bootstrap over an owner
// connection: it is inert unless the authority schema is present (so a daemon
// without the owner bundle applied is unaffected); otherwise it rotates the
// runtime password (two-role only) and registers a fresh per-instance authority
// secret digest. Any owner-connection failure is fail-closed (§9.2) — the
// caller must refuse to serve rather than fall back to a stale credential.
func BootstrapAuthority(ctx context.Context, cfg BootstrapConfig) (BootstrapResult, error) {
	ownerDSN := cfg.OwnerURL
	singleRole := cfg.OwnerURL == ""
	if ownerDSN == "" {
		ownerDSN = cfg.RuntimeURL
	}
	ownerPool, err := Connect(ctx, ownerDSN, cfg.DaemonVersion)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("daemon_pg_owner_bootstrap_failed: cannot reach owner DSN to bootstrap authority: %w", err)
	}
	defer ownerPool.Close()
	owner := ownerPool.Runner

	present, err := authorityRegistryPresent(ctx, owner)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("daemon_pg_owner_bootstrap_failed: cannot read authority schema: %w", err)
	}
	if !present {
		// No owner bundle applied: authority is inert, daemon stays on v2 audit.
		return BootstrapResult{Posture: PostureSchemaAbsent}, nil
	}

	result := BootstrapResult{Posture: PostureSingleRoleSkip}

	if !singleRole {
		newPassword, err := randomToken(32)
		if err != nil {
			return BootstrapResult{}, fmt.Errorf("daemon_pg_owner_bootstrap_failed: generate runtime password: %w", err)
		}
		if err := owner.Exec(ctx, fmt.Sprintf("ALTER ROLE %s PASSWORD %s",
			quoteIdentifier(cfg.RuntimeRole), quoteLiteral(newPassword))); err != nil {
			return BootstrapResult{}, fmt.Errorf("daemon_pg_owner_bootstrap_failed: rotate runtime password: %w", err)
		}
		newURL, err := withURLPassword(cfg.RuntimeURL, newPassword)
		if err != nil {
			return BootstrapResult{}, fmt.Errorf("daemon_pg_owner_bootstrap_failed: rebuild runtime URL: %w", err)
		}
		result.NewRuntimeURL = newURL
		result.Posture = PostureRotated
	}

	secret, err := randomToken(32)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("daemon_pg_owner_bootstrap_failed: generate authority secret: %w", err)
	}
	salt, err := randomToken(16)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("daemon_pg_owner_bootstrap_failed: generate salt: %w", err)
	}
	digest := authorityDigest(secret, salt)
	if err := owner.Exec(ctx, `
		INSERT INTO striatumd.daemon_auth_registry(instance_id, role_name, digest, salt, rotated_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (instance_id, role_name)
		DO UPDATE SET digest = EXCLUDED.digest, salt = EXCLUDED.salt, rotated_at = now()`,
		cfg.InstanceID, cfg.RuntimeRole, digest, salt); err != nil {
		return BootstrapResult{}, fmt.Errorf("daemon_pg_owner_bootstrap_failed: register authority secret: %w", err)
	}

	collision, err := rotatorCollision(ctx, owner, cfg.RuntimeRole, cfg.InstanceID)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("daemon_pg_owner_bootstrap_failed: rotator probe: %w", err)
	}

	result.Secret = secret
	result.Registered = true
	result.RotatorCollision = collision
	return result, nil
}

// authorityDigest computes the registry digest the way assert_daemon_authority
// verifies it: lower-hex sha256 of UTF-8(secret || salt).
func authorityDigest(secret, salt string) string {
	sum := sha256.Sum256([]byte(secret + salt))
	return hex.EncodeToString(sum[:])
}

func authorityRegistryPresent(ctx context.Context, runner Runner) (bool, error) {
	value, err := runner.QueryScalar(ctx,
		"SELECT (to_regclass('striatumd.daemon_auth_registry') IS NOT NULL)::text")
	if err != nil {
		return false, err
	}
	return value == "true", nil
}

// rotatorCollision reports whether another instance recently registered the same
// role (RFC 0110 §9.4 role-scoped single-writer probe). Per-instance roles
// produce distinct role_name values and never collide.
func rotatorCollision(ctx context.Context, runner Runner, roleName, instanceID string) (bool, error) {
	value, err := runner.QueryScalar(ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM striatumd.daemon_auth_registry
		   WHERE role_name = $1 AND instance_id <> $2
		     AND rotated_at > now() - interval '5 minutes'
		)::text`, roleName, instanceID)
	if err != nil {
		return false, err
	}
	return value == "true", nil
}

func randomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func withURLPassword(rawURL, password string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	username := ""
	if parsed.User != nil {
		username = parsed.User.Username()
	}
	parsed.User = url.UserPassword(username, password)
	return parsed.String(), nil
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func quoteLiteral(value string) string {
	return `'` + strings.ReplaceAll(value, `'`, `''`) + `'`
}
