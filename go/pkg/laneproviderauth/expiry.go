package laneproviderauth

// RFC 0162 Layer 1 — provider-specific expiry extraction (the honest boundary).
//
// The sampler parses expiry per credential shape:
//   - codex OAuth: the tokens.id_token JWT `exp` (or a tokens.expiry / expires_at
//     field when present);
//   - claude OAuth: the claudeAiOauth.expiresAt millisecond timestamp (or a
//     top-level expiresAt).
//
// kind="api_key" credentials have NO expiry → ParseExpiry reports HasExpiry=false
// so they emit no seconds_to_expiry series; they rely on the census
// (sample_present) for absence detection. This gap is explicit, not silent.

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// ErrUnparseableCredential is returned when a present credential payload cannot
// be parsed as the provider's expected shape. The sampler treats this as a
// fail-closed resolver_mismatch (a credential we cannot prove is not green).
var ErrUnparseableCredential = errors.New("lane credential payload is not parseable")

// CredentialExpiry is the extracted timing of a sampled credential. HasExpiry is
// false for api_key credentials and for an OAuth payload that genuinely carries
// none; in both cases ExpiresAt is the zero time and no expiry series is emitted.
type CredentialExpiry struct {
	HasExpiry bool
	ExpiresAt time.Time
	IssuedAt  time.Time
}

// ParseExpiry extracts the credential's expiry (and issue time when available)
// from its file payload, per provider/kind. An api_key payload yields
// HasExpiry=false with no error. A present-but-unparseable OAuth payload yields
// ErrUnparseableCredential so the caller can fail closed.
func ParseExpiry(provider, kind string, payload []byte) (CredentialExpiry, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == KindAPIKey {
		// api_key credentials have no expiry; a parseable presence is enough.
		if len(strings.TrimSpace(string(payload))) == 0 {
			return CredentialExpiry{}, ErrUnparseableCredential
		}
		return CredentialExpiry{HasExpiry: false}, nil
	}

	var doc map[string]any
	if err := json.Unmarshal(payload, &doc); err != nil {
		return CredentialExpiry{}, ErrUnparseableCredential
	}

	switch provider {
	case ProviderCodex:
		return codexExpiry(doc)
	case ProviderClaude:
		return claudeExpiry(doc)
	default:
		return CredentialExpiry{}, ErrUnparseableCredential
	}
}

func codexExpiry(doc map[string]any) (CredentialExpiry, error) {
	tokens, _ := doc["tokens"].(map[string]any)
	out := CredentialExpiry{}
	if tokens != nil {
		if when, ok := jwtExp(stringField(tokens, "id_token")); ok {
			out.HasExpiry = true
			out.ExpiresAt = when
		}
		if !out.HasExpiry {
			if when, ok := parseTimeValue(tokens["expiry"]); ok {
				out.HasExpiry = true
				out.ExpiresAt = when
			}
		}
	}
	if !out.HasExpiry {
		if when, ok := parseTimeValue(doc["expires_at"]); ok {
			out.HasExpiry = true
			out.ExpiresAt = when
		}
	}
	if when, ok := parseTimeValue(doc["last_refresh"]); ok {
		out.IssuedAt = when
	}
	if !out.HasExpiry {
		// A parseable codex auth.json with no recognizable expiry field is still a
		// valid sample (e.g. an api-key-style codex login); report presence without
		// expiry rather than failing closed.
		return CredentialExpiry{HasExpiry: false}, nil
	}
	return out, nil
}

func claudeExpiry(doc map[string]any) (CredentialExpiry, error) {
	out := CredentialExpiry{}
	if oauth, ok := doc["claudeAiOauth"].(map[string]any); ok {
		if when, ok := parseTimeValue(oauth["expiresAt"]); ok {
			out.HasExpiry = true
			out.ExpiresAt = when
		}
	}
	if !out.HasExpiry {
		if when, ok := parseTimeValue(doc["expiresAt"]); ok {
			out.HasExpiry = true
			out.ExpiresAt = when
		}
	}
	if !out.HasExpiry {
		return CredentialExpiry{HasExpiry: false}, nil
	}
	return out, nil
}

func stringField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// jwtExp decodes the `exp` claim (seconds since epoch) from a JWT without
// verifying the signature — expiry is a read-only observation, never a trust
// decision. Returns ok=false when the token is empty, malformed, or carries no
// numeric exp.
func jwtExp(token string) (time.Time, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return time.Time{}, false
	}
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return time.Time{}, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Tolerate padded base64url too.
		raw, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return time.Time{}, false
		}
	}
	var claims map[string]any
	if err := json.Unmarshal(raw, &claims); err != nil {
		return time.Time{}, false
	}
	return parseTimeValue(claims["exp"])
}

// parseTimeValue interprets a credential timestamp field. It accepts a numeric
// epoch (seconds for ≤1e12, milliseconds above), an RFC3339 string, or a numeric
// string. Returns ok=false for an absent/zero/unrecognized value.
func parseTimeValue(v any) (time.Time, bool) {
	switch t := v.(type) {
	case float64:
		return epochToTime(t)
	case json.Number:
		if f, err := t.Float64(); err == nil {
			return epochToTime(f)
		}
		return time.Time{}, false
	case int64:
		return epochToTime(float64(t))
	case int:
		return epochToTime(float64(t))
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return time.Time{}, false
		}
		if when, err := time.Parse(time.RFC3339, s); err == nil {
			return when.UTC(), true
		}
		// Numeric string (epoch).
		var f float64
		if _, err := jsonUnmarshalNumber(s, &f); err == nil {
			return epochToTime(f)
		}
		return time.Time{}, false
	default:
		return time.Time{}, false
	}
}

func jsonUnmarshalNumber(s string, out *float64) (bool, error) {
	if err := json.Unmarshal([]byte(s), out); err != nil {
		return false, err
	}
	return true, nil
}

// epochToTime converts a numeric epoch to a UTC time, treating values at or above
// 1e12 as milliseconds (claude's expiresAt is in ms; codex JWT exp is in s).
func epochToTime(v float64) (time.Time, bool) {
	if v <= 0 {
		return time.Time{}, false
	}
	if v >= 1e12 {
		sec := int64(v / 1000)
		nsec := int64((v - float64(sec)*1000) * 1e6)
		return time.Unix(sec, nsec).UTC(), true
	}
	sec := int64(v)
	nsec := int64((v - float64(sec)) * 1e9)
	return time.Unix(sec, nsec).UTC(), true
}
