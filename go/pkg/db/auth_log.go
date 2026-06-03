package db

import "regexp"

// AuthLogEvent enumerates the daemon_auth_log event kinds (RFC 0110 §12,
// C-AUTH-LOG-PRIVACY). The log records auth-posture transitions; it never
// records a secret or DSN value.
type AuthLogEvent string

const (
	AuthLogBootstrap            AuthLogEvent = "bootstrap"
	AuthLogRotated              AuthLogEvent = "rotated"
	AuthLogRotationSkipped      AuthLogEvent = "rotation_skipped_single_role"
	AuthLogRotationFailed       AuthLogEvent = "rotation_failed"
	AuthLogOwnerBootstrapFailed AuthLogEvent = "owner_bootstrap_failed"
)

// authLogAllowedKeys is the strict whitelist for daemon_auth_log.detail. Any key
// outside this set is dropped before insertion, so a raw driver error map or DSN
// cannot ride into the log through an unexpected field.
var authLogAllowedKeys = map[string]bool{
	"event":          true,
	"daemon_version": true,
	"instance_id":    true,
	"reason_code":    true,
	"duration_ms":    true,
}

var (
	dsnPattern    = regexp.MustCompile(`(?i)postgres(?:ql)?://[^\s"']*`)
	credKVPattern = regexp.MustCompile(`(?i)\b(password|passwd|secret|token|sslpassword)\s*=\s*[^\s"';&]+`)
)

// RedactAuthLogDetail produces the detail jsonb written to daemon_auth_log: it
// keeps only whitelisted keys and runs every string value through the
// DSN/credential redactor, so neither a secret nor a connection string can
// survive insertion (T-AUTHLOG-REDACT). It is the single writer path for the
// detail column. The DB insert that consumes this is inert until release N+1
// creates the owner-owned table; the redactor lands now so it is gate-covered.
func RedactAuthLogDetail(detail map[string]any) map[string]any {
	out := make(map[string]any, len(detail))
	for key, value := range detail {
		if !authLogAllowedKeys[key] {
			continue
		}
		out[key] = redactAuthLogValue(value)
	}
	return out
}

func redactAuthLogValue(value any) any {
	text, ok := value.(string)
	if !ok {
		return value
	}
	return redactSecrets(text)
}

// redactSecrets removes connection strings and credential key/value pairs from
// free text (e.g. a raw driver error that embedded the DSN).
func redactSecrets(text string) string {
	text = dsnPattern.ReplaceAllString(text, "<redacted-dsn>")
	text = credKVPattern.ReplaceAllString(text, "$1=<redacted>")
	return text
}
