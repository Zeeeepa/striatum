package laneproviderauth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"
)

// TestThresholdFromRosterNotObserved is FA-4: the per-lane thresholds derive from
// the DECLARED roster, never from observed credential lifetime — a degrading
// credential cannot shrink its own goalposts. The declared value is used verbatim
// when present; the fallback default derives only from the declared renewal
// cadence, not from any sampled expiry.
func TestThresholdFromRosterNotObserved(t *testing.T) {
	declared := RosterEntry{
		Lane: "codex", Provider: ProviderCodex, Kind: KindOAuth,
		StalenessThresholdSeconds: 9000,
		ExpiryLeadSeconds:         3600,
		RenewalCadenceSeconds:     1200,
	}
	if got := declared.EffectiveStalenessThresholdSeconds(); got != 9000 {
		t.Fatalf("declared staleness threshold = %v; want 9000 (roster value, not derived)", got)
	}
	if got := declared.EffectiveExpiryLeadSeconds(); got != 3600 {
		t.Fatalf("declared expiry lead = %v; want 3600 (roster value)", got)
	}

	// With no explicit thresholds the default derives from the DECLARED cadence
	// (1.5×/3×), still independent of any observed/sampled lifetime.
	defaulted := RosterEntry{Lane: "claude", Provider: ProviderClaude, Kind: KindOAuth, RenewalCadenceSeconds: 1000}
	if got := defaulted.EffectiveStalenessThresholdSeconds(); got != 1500 {
		t.Fatalf("default staleness threshold = %v; want 1500 (1.5× declared cadence)", got)
	}
	if got := defaulted.EffectiveExpiryLeadSeconds(); got != 3000 {
		t.Fatalf("default expiry lead = %v; want 3000 (3× declared cadence)", got)
	}
}

// TestCredResolverFailsClosedOnUnprovenSource is FA-F2: a provider with no
// resolver entry, or a launch env from which no credential path can be resolved,
// fails closed into ErrResolverMismatch rather than guessing a (decoy) path.
func TestCredResolverFailsClosedOnUnprovenSource(t *testing.T) {
	// Unknown provider — no resolver entry.
	if _, err := ResolveCredential("mystery", KindOAuth, []string{"HOME=/home/lane"}); !errors.Is(err, ErrResolverMismatch) {
		t.Fatalf("unknown provider: err = %v; want ErrResolverMismatch", err)
	}
	// claude with neither CLAUDE_CONFIG_DIR nor HOME — no resolvable path.
	if _, err := ResolveCredential(ProviderClaude, KindOAuth, []string{"TERM=xterm"}); !errors.Is(err, ErrResolverMismatch) {
		t.Fatalf("claude with no home/config: err = %v; want ErrResolverMismatch", err)
	}
	// codex with neither CODEX_HOME nor HOME.
	if _, err := ResolveCredential(ProviderCodex, KindOAuth, nil); !errors.Is(err, ErrResolverMismatch) {
		t.Fatalf("codex with empty env: err = %v; want ErrResolverMismatch", err)
	}
}

// TestCredResolverTracksLaunchEnvNotHomeDecoy is FA-F2: when the lane's launch
// env names a provider config-dir, the resolver reads THAT credential, never the
// (possibly fresher) credential a bare HOME would resolve.
func TestCredResolverTracksLaunchEnvNotHomeDecoy(t *testing.T) {
	// claude: CLAUDE_CONFIG_DIR wins over HOME.
	claude, err := ResolveCredential(ProviderClaude, KindOAuth, []string{
		"CLAUDE_CONFIG_DIR=/run/lane/claude-conf",
		"HOME=/home/decoy",
	})
	if err != nil {
		t.Fatalf("claude resolve: %v", err)
	}
	if claude.EnvKey != EnvClaudeConfigDir {
		t.Fatalf("claude resolved via %q; want %q (launch env, not HOME)", claude.EnvKey, EnvClaudeConfigDir)
	}
	if want := "/run/lane/claude-conf/" + ClaudeCredentialFileName; claude.Path != want {
		t.Fatalf("claude path = %q; want %q (launch-env dir, not HOME decoy)", claude.Path, want)
	}

	secure, err := ResolveCredential(ProviderClaude, KindOAuth, []string{
		"CLAUDE_SECURESTORAGE_CONFIG_DIR=/run/lane/securestorage",
		"CLAUDE_CONFIG_DIR=/run/lane/claude-conf",
		"HOME=/home/decoy",
	})
	if err != nil {
		t.Fatalf("claude secure-storage resolve: %v", err)
	}
	if secure.EnvKey != EnvClaudeSecureStorageConfigDir {
		t.Fatalf("claude secure-storage resolved via %q; want %q", secure.EnvKey, EnvClaudeSecureStorageConfigDir)
	}
	if want := "/run/lane/securestorage/" + ClaudeCredentialFileName; secure.Path != want {
		t.Fatalf("secure-storage path = %q; want %q", secure.Path, want)
	}

	// codex: CODEX_HOME wins over HOME/.codex.
	codex, err := ResolveCredential(ProviderCodex, KindOAuth, []string{
		"CODEX_HOME=/run/lane/codex-conf",
		"HOME=/home/decoy",
	})
	if err != nil {
		t.Fatalf("codex resolve: %v", err)
	}
	if codex.EnvKey != EnvCodexHome {
		t.Fatalf("codex resolved via %q; want %q (launch env, not HOME)", codex.EnvKey, EnvCodexHome)
	}
	if want := "/run/lane/codex-conf/" + CodexAuthFileName; codex.Path != want {
		t.Fatalf("codex path = %q; want %q", codex.Path, want)
	}
}

// TestCredExpirySamplerReadsLanePresentedCredential is FA-F2/FA-6: the sampler
// reads the SAME credential the lane presents at runtime (the launch-env-resolved
// path), parses its expiry, and never tracks a fresher HOME decoy. It also proves
// the absent / mismatch / api_key branches.
func TestCredExpirySamplerReadsLanePresentedCredential(t *testing.T) {
	expires := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	presented := claudeCredentialBytes(t, expires)
	decoyExpires := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	decoy := claudeCredentialBytes(t, decoyExpires)

	presentedPath := "/run/lane/claude-conf/" + ClaudeCredentialFileName
	decoyPath := "/home/decoy/.claude/" + ClaudeCredentialFileName
	read := func(path string) ([]byte, error) {
		switch path {
		case presentedPath:
			return presented, nil
		case decoyPath:
			return decoy, nil
		default:
			return nil, os.ErrNotExist
		}
	}

	entry := RosterEntry{Lane: "claude", Provider: ProviderClaude, Kind: KindOAuth, LaunchEnv: map[string]string{}}
	launchEnv := []string{"CLAUDE_CONFIG_DIR=/run/lane/claude-conf", "HOME=/home/decoy"}

	sample, err := SampleLaneCredential(entry, launchEnv, read)
	if err != nil {
		t.Fatalf("sample: %v", err)
	}
	if !sample.HasExpiry || !sample.ExpiresAt.Equal(expires) {
		t.Fatalf("sample expiry = (%v, %v); want the presented credential's %v, not the decoy's %v",
			sample.HasExpiry, sample.ExpiresAt, expires, decoyExpires)
	}

	// Absent credential → ErrCredentialAbsent (census absence, not a green gauge).
	if _, err := SampleLaneCredential(entry, []string{"CLAUDE_CONFIG_DIR=/nowhere", "HOME=/nowhere"}, read); !errors.Is(err, ErrCredentialAbsent) {
		t.Fatalf("absent credential: err = %v; want ErrCredentialAbsent", err)
	}

	// Unprovable source → ErrResolverMismatch (fail closed).
	if _, err := SampleLaneCredential(RosterEntry{Lane: "agy", Provider: "agy", Kind: KindAPIKey}, []string{"HOME=/home/decoy"}, read); !errors.Is(err, ErrResolverMismatch) {
		t.Fatalf("unprovable source: err = %v; want ErrResolverMismatch", err)
	}

	// api_key with a present credential → sample with HasExpiry=false.
	apiRead := func(path string) ([]byte, error) { return []byte(`{"api_key":"present"}`), nil }
	apiEntry := RosterEntry{Lane: "codex", Provider: ProviderCodex, Kind: KindAPIKey}
	apiSample, err := SampleLaneCredential(apiEntry, []string{"HOME=/home/lane"}, apiRead)
	if err != nil {
		t.Fatalf("api_key sample: %v", err)
	}
	if apiSample.HasExpiry {
		t.Fatalf("api_key sample must not carry expiry; got %v", apiSample.ExpiresAt)
	}
}

// TestParseExpiryShapes covers the codex JWT exp and claude expiresAt-ms shapes.
func TestParseExpiryShapes(t *testing.T) {
	exp := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)

	codexDoc := map[string]any{"tokens": map[string]any{"id_token": makeJWT(t, exp.Unix())}}
	codexBytes, _ := json.Marshal(codexDoc)
	got, err := ParseExpiry(ProviderCodex, KindOAuth, codexBytes)
	if err != nil {
		t.Fatalf("codex parse: %v", err)
	}
	if !got.HasExpiry || !got.ExpiresAt.Equal(exp) {
		t.Fatalf("codex expiry = (%v,%v); want %v", got.HasExpiry, got.ExpiresAt, exp)
	}

	claudeBytes := claudeCredentialBytes(t, exp)
	gotC, err := ParseExpiry(ProviderClaude, KindOAuth, claudeBytes)
	if err != nil {
		t.Fatalf("claude parse: %v", err)
	}
	if !gotC.HasExpiry || !gotC.ExpiresAt.Equal(exp) {
		t.Fatalf("claude expiry = (%v,%v); want %v", gotC.HasExpiry, gotC.ExpiresAt, exp)
	}

	// Unparseable OAuth payload fails closed.
	if _, err := ParseExpiry(ProviderClaude, KindOAuth, []byte("not json")); !errors.Is(err, ErrUnparseableCredential) {
		t.Fatalf("unparseable: err = %v; want ErrUnparseableCredential", err)
	}
}

// TestParseRosterNormalizes proves the loader drops blank/invalid-kind entries
// and sorts by lane, so an unbounded kind can never reach the wire.
func TestParseRosterNormalizes(t *testing.T) {
	roster, err := ParseRoster([]byte(`{"lanes":[
		{"lane":"codex","provider":"Codex","kind":"OAuth"},
		{"lane":"","provider":"claude","kind":"oauth"},
		{"lane":"agy","provider":"agy","kind":"bogus"},
		{"lane":"claude","provider":"claude","kind":"oauth"}
	]}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(roster.Entries) != 2 {
		t.Fatalf("entries = %d (%v); want 2 (blank lane + bogus kind dropped)", len(roster.Entries), roster.Entries)
	}
	if roster.Entries[0].Lane != "claude" || roster.Entries[1].Lane != "codex" {
		t.Fatalf("entries not sorted by lane: %v", roster.Entries)
	}
	if roster.Entries[1].Provider != "codex" || roster.Entries[1].Kind != KindOAuth {
		t.Fatalf("codex entry not normalized: %#v", roster.Entries[1])
	}
}

func claudeCredentialBytes(t *testing.T, expires time.Time) []byte {
	t.Helper()
	doc := map[string]any{
		"claudeAiOauth": map[string]any{
			"accessToken": "tok",
			"expiresAt":   expires.UnixMilli(),
		},
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal claude cred: %v", err)
	}
	return b
}

func makeJWT(t *testing.T, exp int64) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	claims, err := json.Marshal(map[string]any{"exp": exp})
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	body := base64.RawURLEncoding.EncodeToString(claims)
	return header + "." + body + ".sig"
}
