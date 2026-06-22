package laneproviderauth

// RFC 0162 Backbone roster — the operator-declared denominator + thresholds +
// resolver hints for the lane-auth silent-failure observability MVP.
//
// The roster is the census *expected set*: one declared entry per lane auth
// mechanism, host-level, keyed by the lane OS user (`Lane`). It folds to
// striatum_lane_auth_expected{lane,provider,kind} and the OQ4 threshold gauges,
// and it is the F2 drift cross-check (`CredentialPathTemplate` is a
// declared-vs-resolver reconciliation signal, NEVER the authoritative read
// path). It carries NO secret — only the closed-enum provider/kind, the declared
// path template, the operator-declared thresholds, and an optional launch-env
// hint map the resolver consults.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Credential kinds — the closed enum for the lane-auth observability surface
// (RFC 0162 OQ3). A lane's credential is either an EXPIRING OAuth token (which
// carries positive expiry telemetry) or a non-expiring api_key (census-covered
// only — an explicit accepted/deferred risk for positive validity, F1 fold).
const (
	KindOAuth  = "oauth"
	KindAPIKey = "api_key"
)

// IsCredentialKind reports whether kind is a recognized closed-enum credential
// kind. The roster loader drops entries whose kind is outside this set so an
// unbounded `kind` value can never reach the wire.
func IsCredentialKind(kind string) bool {
	switch kind {
	case KindOAuth, KindAPIKey:
		return true
	}
	return false
}

// RosterEntry is one operator-declared lane auth mechanism. `Lane` is the lane OS
// user (the roster slug, a small non-private host-level set). `Provider`/`Kind`
// are closed enums. The thresholds are operator-declared (OQ4) — exported as
// gauges, NEVER auto-derived from observed credential lifetime (a degrading
// credential must not move its own goalposts, FA-4). `LaunchEnv` carries the
// optional credential-home / config-dir env keys the resolver consults so the
// sampler reads the credential the live lane process resolves (F2).
type RosterEntry struct {
	Lane                      string            `json:"lane"`
	Provider                  string            `json:"provider"`
	Kind                      string            `json:"kind"`
	CredentialPathTemplate    string            `json:"credential_path_template,omitempty"`
	StalenessThresholdSeconds float64           `json:"staleness_threshold_seconds,omitempty"`
	ExpiryLeadSeconds         float64           `json:"expiry_lead_seconds,omitempty"`
	RenewalCadenceSeconds     float64           `json:"renewal_cadence_seconds,omitempty"`
	LaunchEnv                 map[string]string `json:"launch_env,omitempty"`
}

// EffectiveStalenessThresholdSeconds returns the Layer 3 heartbeat staleness
// threshold for this lane: the operator-declared value when present, else a
// sensible default derived from the DECLARED renewal cadence (≈1.5× cadence) so a
// newly rostered lane is covered without hand-tuning. The default is derived from
// the declared roster, never from observed lifetime (FA-4).
func (e RosterEntry) EffectiveStalenessThresholdSeconds() float64 {
	if e.StalenessThresholdSeconds > 0 {
		return e.StalenessThresholdSeconds
	}
	if e.RenewalCadenceSeconds > 0 {
		return e.RenewalCadenceSeconds * 1.5
	}
	return 0
}

// EffectiveExpiryLeadSeconds returns the Layer 1 expiry tripwire lead for this
// lane: the operator-declared value when present, else ≈3× the declared renewal
// cadence. Like the staleness threshold it derives only from the declared roster
// (FA-4), never from observed lifetime.
func (e RosterEntry) EffectiveExpiryLeadSeconds() float64 {
	if e.ExpiryLeadSeconds > 0 {
		return e.ExpiryLeadSeconds
	}
	if e.RenewalCadenceSeconds > 0 {
		return e.RenewalCadenceSeconds * 3
	}
	return 0
}

// Roster is the full declared set of lane auth mechanisms, normalized and sorted
// by lane.
type Roster struct {
	Entries []RosterEntry
}

// rosterFile is the on-disk JSON shape: a bare array of entries OR an object with
// a `lanes` array. Both decode here so an operator can write whichever reads
// cleaner.
type rosterFile struct {
	Lanes []RosterEntry `json:"lanes"`
}

// LoadRoster reads and normalizes the roster from path. A missing file yields an
// empty roster and no error (the roster is operator-optional; absence simply
// means no expected lanes and so no census denominator). Malformed JSON is an
// error. Entries with a blank lane/provider or a kind outside the closed enum are
// dropped so a typo can never widen the wire contract.
func LoadRoster(path string) (Roster, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return Roster{}, nil
	}
	raw, err := os.ReadFile(path) //nolint:gosec // operator-declared daemon config path
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Roster{}, nil
		}
		return Roster{}, fmt.Errorf("read lane-auth roster %s: %w", path, err)
	}
	return ParseRoster(raw)
}

// ParseRoster normalizes a roster JSON payload (bare array or {"lanes":[…]}).
func ParseRoster(raw []byte) (Roster, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return Roster{}, nil
	}
	var entries []RosterEntry
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal(raw, &entries); err != nil {
			return Roster{}, fmt.Errorf("parse lane-auth roster: %w", err)
		}
	} else {
		var file rosterFile
		if err := json.Unmarshal(raw, &file); err != nil {
			return Roster{}, fmt.Errorf("parse lane-auth roster: %w", err)
		}
		entries = file.Lanes
	}
	return normalizeRoster(entries), nil
}

func normalizeRoster(entries []RosterEntry) Roster {
	seen := map[string]bool{}
	out := make([]RosterEntry, 0, len(entries))
	for _, e := range entries {
		e.Lane = strings.TrimSpace(e.Lane)
		e.Provider = strings.ToLower(strings.TrimSpace(e.Provider))
		e.Kind = strings.ToLower(strings.TrimSpace(e.Kind))
		if e.Lane == "" || e.Provider == "" || !IsCredentialKind(e.Kind) {
			continue
		}
		if seen[e.Lane] {
			continue
		}
		seen[e.Lane] = true
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Lane < out[j].Lane })
	return Roster{Entries: out}
}

// DefaultRosterPath resolves the daemon-config roster path: the explicit
// STRIATUM_LANE_AUTH_ROSTER override if set, else lane-auth-roster.json under the
// XDG config dir (defaulting to ~/.config/striatum/). It returns "" when no home
// can be resolved, in which case LoadRoster("") yields an empty roster.
func DefaultRosterPath() string {
	if override := strings.TrimSpace(os.Getenv("STRIATUM_LANE_AUTH_ROSTER")); override != "" {
		return override
	}
	if dir := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); dir != "" {
		return filepath.Join(dir, "striatum", "lane-auth-roster.json")
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ""
	}
	return filepath.Join(home, ".config", "striatum", "lane-auth-roster.json")
}
