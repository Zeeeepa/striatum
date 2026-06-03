package db

import (
	"strings"
	"testing"
)

// TestAuthLogRedaction is T-AUTHLOG-REDACT (RFC 0110 §12, C-AUTH-LOG-PRIVACY):
// the daemon_auth_log detail writer keeps only whitelisted keys and lets no
// secret or DSN substring survive insertion.
func TestAuthLogRedaction(t *testing.T) {
	detail := map[string]any{
		"event":          string(AuthLogRotationFailed),
		"daemon_version": "v2.14.0",
		"instance_id":    "inst-1",
		"reason_code":    `dial error to postgresql://striatumd_rw:s3cr3tP@ss@/striatum?password=s3cr3tP@ss`,
		"duration_ms":    12,
		// Non-whitelisted keys that must be dropped entirely.
		"dsn":          "postgres://striatumd_rw:hunter2@/striatum",
		"raw_error":    "FATAL: password authentication failed for user with secret=hunter2",
		"auth_secret":  "deadbeefsecret",
		"stack":        "goroutine 1 ...",
	}

	out := RedactAuthLogDetail(detail)

	// Whitelist enforced: only the five allowed keys remain.
	for key := range out {
		if !authLogAllowedKeys[key] {
			t.Errorf("non-whitelisted key %q survived redaction", key)
		}
	}
	for _, want := range []string{"event", "daemon_version", "instance_id", "reason_code", "duration_ms"} {
		if _, ok := out[want]; !ok {
			t.Errorf("whitelisted key %q was dropped", want)
		}
	}
	for _, dropped := range []string{"dsn", "raw_error", "auth_secret", "stack"} {
		if _, ok := out[dropped]; ok {
			t.Errorf("non-whitelisted key %q must be dropped", dropped)
		}
	}

	// No secret/DSN substring survives anywhere in the redacted detail.
	blob := strings.ToLower(stringifyValues(out))
	for _, secret := range []string{"s3cr3t", "hunter2", "deadbeefsecret", "postgres://", "postgresql://"} {
		if strings.Contains(blob, strings.ToLower(secret)) {
			t.Errorf("redacted detail still contains %q: %s", secret, blob)
		}
	}
}

func stringifyValues(m map[string]any) string {
	var b strings.Builder
	for _, v := range m {
		if s, ok := v.(string); ok {
			b.WriteString(s)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func TestCapabilityParityInertAndBothDirections(t *testing.T) {
	// Inert (release N): no authority schema -> pass even with required caps.
	if err := checkCapabilityParity([]string{"cap.audit"}, nil, nil, false); err != nil {
		t.Fatalf("inert checker (no authority schema) must pass: %v", err)
	}

	// new-binary / old-schema: a required cap is not stamped -> refuse.
	if err := checkCapabilityParity([]string{"cap.audit"}, []string{"cap.audit"}, nil, true); err == nil {
		t.Fatal("a required-but-unstamped capability must fail closed")
	}

	// old-binary / authority-schema: a stamped cap the binary does not support -> refuse.
	if err := checkCapabilityParity(nil, nil, []string{"cap.audit"}, true); err == nil {
		t.Fatal("a stamped-but-unsupported capability must fail closed")
	}

	// Parity in both directions: pass.
	if err := checkCapabilityParity([]string{"cap.audit"}, []string{"cap.audit"}, []string{"cap.audit"}, true); err != nil {
		t.Fatalf("matched parity must pass: %v", err)
	}
}
