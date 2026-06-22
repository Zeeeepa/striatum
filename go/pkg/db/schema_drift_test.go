package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// driftRunner is a canned Runner for the RFC 0142 Layer 3 part 1 drift gate read
// path (LiveFingerprint / CheckSchemaDrift). It answers the two queries
// LiveFingerprint issues — the to_regclass presence probe and the singleton-row
// fingerprint SELECT — and records every statement so a test can assert the gate
// is a PURE READ (the drift check must leave the database untouched). The
// self-record path (RecordSchemaFingerprint) is exercised separately against a
// live database in the PG-gated tests; this fake exists for the no-PG unit logic.
type driftRunner struct {
	statePresent bool   // schema_state table exists
	liveValue    string // recorded fingerprint value
	scalarErr    error
	queries      []string
}

func (r *driftRunner) Exec(_ context.Context, sql string, _ ...any) error {
	r.queries = append(r.queries, sql)
	return errors.New("driftRunner: Exec must not be called by the read-only drift check")
}

func (r *driftRunner) QueryRow(context.Context, string, ...any) Row { return nil }

func (r *driftRunner) QueryScalar(_ context.Context, sql string, _ ...any) (string, error) {
	r.queries = append(r.queries, sql)
	if r.scalarErr != nil {
		return "", r.scalarErr
	}
	if strings.Contains(sql, "to_regclass") {
		if r.statePresent {
			return "true", nil
		}
		return "false", nil
	}
	if strings.Contains(sql, "FROM striatumd.schema_state") {
		return r.liveValue, nil
	}
	return "", fmt.Errorf("driftRunner: unexpected query %q", sql)
}

func (r *driftRunner) BeginTx(context.Context) (TxRunner, error) {
	return nil, errors.New("driftRunner: BeginTx must not be called by the read-only drift check")
}

func (r *driftRunner) wroteAnything() bool {
	for _, q := range r.queries {
		upper := strings.ToUpper(q)
		if strings.Contains(upper, "INSERT") || strings.Contains(upper, "UPDATE") ||
			strings.Contains(upper, "DELETE") || strings.Contains(upper, "ALTER") ||
			strings.Contains(upper, "CREATE") || strings.Contains(upper, "DROP") {
			return true
		}
	}
	return false
}

// TestExpectedFingerprintIsDeterministic pins the RFC 0142 Layer 3 part 1 schema
// fingerprint contract: two computations from the same embedded source produce the
// SAME 64-hex digest, so it is stable across processes (the drift gate's whole
// premise). A non-deterministic fingerprint would false-halt a healthy daemon.
func TestExpectedFingerprintIsDeterministic(t *testing.T) {
	a, err := ExpectedFingerprint()
	if err != nil {
		t.Fatalf("ExpectedFingerprint: %v", err)
	}
	b, err := ExpectedFingerprint()
	if err != nil {
		t.Fatalf("ExpectedFingerprint (second): %v", err)
	}
	if a != b {
		t.Fatalf("ExpectedFingerprint not deterministic: %q vs %q", a, b)
	}
	if len(a) != 64 {
		t.Fatalf("ExpectedFingerprint length = %d, want 64 (sha256 hex)", len(a))
	}
	for _, c := range a {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Fatalf("ExpectedFingerprint is not lower-hex: %q", a)
		}
	}
}

// TestComposeFingerprintIsSensitive proves the digest CHANGES when the
// migration/owner-bundle set changes — the other half of the contract. We use the
// pure composeFingerprint helper (no embed FS) so we can mutate the set directly:
// a changed migration hash, an added migration, an added owner bundle, and a
// migration↔bundle swap must each produce a DIFFERENT digest, and an unrelated
// re-ordering of the input map must NOT (order-stability).
func TestComposeFingerprintIsSensitive(t *testing.T) {
	migs := map[string]string{
		"0001_a.sql": "aaaa",
		"0002_b.sql": "bbbb",
	}
	bundles := map[string]string{
		"0001": "1111",
		"0002": "2222",
	}
	base := composeFingerprint(migs, bundles)

	// Same content, different map insertion order → identical digest (order-stable).
	migs2 := map[string]string{
		"0002_b.sql": "bbbb",
		"0001_a.sql": "aaaa",
	}
	bundles2 := map[string]string{
		"0002": "2222",
		"0001": "1111",
	}
	if got := composeFingerprint(migs2, bundles2); got != base {
		t.Fatalf("composeFingerprint not order-stable: %q vs %q", got, base)
	}

	// Changed migration hash → different digest.
	changedMig := map[string]string{"0001_a.sql": "AAAA", "0002_b.sql": "bbbb"}
	if got := composeFingerprint(changedMig, bundles); got == base {
		t.Fatal("composeFingerprint did not change when a migration hash changed")
	}

	// Added migration → different digest.
	addedMig := map[string]string{"0001_a.sql": "aaaa", "0002_b.sql": "bbbb", "0003_c.sql": "cccc"}
	if got := composeFingerprint(addedMig, bundles); got == base {
		t.Fatal("composeFingerprint did not change when a migration was added")
	}

	// Added owner bundle → different digest.
	addedBundle := map[string]string{"0001": "1111", "0002": "2222", "0003": "3333"}
	if got := composeFingerprint(migs, addedBundle); got == base {
		t.Fatal("composeFingerprint did not change when an owner bundle was added")
	}

	// A migration and a bundle with the SAME key+hash must not collide across the
	// section boundary: moving a step from migrations to bundles changes the digest.
	swapMig := map[string]string{"0001_a.sql": "aaaa"}
	swapBundle := map[string]string{"0001": "1111", "0002": "2222", "0002_b.sql": "bbbb"}
	if got := composeFingerprint(swapMig, swapBundle); got == base {
		t.Fatal("composeFingerprint collided a migration with an owner bundle across the section boundary")
	}
}

// TestEvaluateSchemaDrift covers the pure comparison policy: equal = in-sync,
// divergent = drift, and an EMPTY live (not yet recorded / pre-P3 database) is
// NEVER drift, so an older database upgrades cleanly.
func TestEvaluateSchemaDrift(t *testing.T) {
	cases := []struct {
		name      string
		expected  string
		live      string
		wantDrift bool
	}{
		{"in_sync", "abc", "abc", false},
		{"drift", "abc", "xyz", true},
		{"unrecorded_empty_live", "abc", "", false},
		{"both_empty", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := EvaluateSchemaDrift(tc.expected, tc.live)
			if d.Drift != tc.wantDrift {
				t.Fatalf("EvaluateSchemaDrift(%q,%q).Drift = %v; want %v", tc.expected, tc.live, d.Drift, tc.wantDrift)
			}
			if d.Expected != tc.expected || d.Live != tc.live {
				t.Fatalf("EvaluateSchemaDrift did not echo inputs: got %+v", d)
			}
		})
	}
}

// TestCheckSchemaDriftShadowVsRefuse is the heart of P3: given a fake LIVE value,
// the gate must (a) be a pure read, (b) in DEFAULT shadow mode return drift=true
// with NO error (boot continues), and (c) only when STRIATUM_SCHEMA_DRIFT_REFUSE
// is set return the typed *SchemaDriftError (refuse to serve). The healthy
// (in-sync) and unrecorded cases never drift in either mode.
func TestCheckSchemaDriftShadowVsRefuse(t *testing.T) {
	expected, err := ExpectedFingerprint()
	if err != nil {
		t.Fatalf("ExpectedFingerprint: %v", err)
	}
	const driftedLive = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef0"

	cases := []struct {
		name      string
		present   bool
		live      string
		refuse    bool
		wantDrift bool
		wantErr   bool
		wantTyped bool
	}{
		{"in_sync_shadow", true, expected, false, false, false, false},
		{"in_sync_refuse", true, expected, true, false, false, false},
		{"unrecorded_shadow", false, "", false, false, false, false},
		{"unrecorded_refuse", true, "", true, false, false, false},
		{"drift_shadow_continues", true, driftedLive, false, true, false, false},
		{"drift_refuse_halts", true, driftedLive, true, true, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.refuse {
				t.Setenv(EnvSchemaDriftRefuse, "1")
			} else {
				t.Setenv(EnvSchemaDriftRefuse, "")
			}
			runner := &driftRunner{statePresent: tc.present, liveValue: tc.live}
			drifted, err := CheckSchemaDrift(context.Background(), runner)

			// The gate must NEVER write, regardless of outcome.
			if runner.wroteAnything() {
				t.Fatalf("CheckSchemaDrift issued a write; it must be read-only: %v", runner.queries)
			}
			if drifted != tc.wantDrift {
				t.Fatalf("drift = %v; want %v", drifted, tc.wantDrift)
			}
			if tc.wantErr {
				if err == nil {
					t.Fatal("want a refuse-to-serve halt, got nil error")
				}
				if !errors.Is(err, ErrSchemaDrift) {
					t.Fatalf("refuse error must wrap ErrSchemaDrift, got %v", err)
				}
				var typed *SchemaDriftError
				if tc.wantTyped && !errors.As(err, &typed) {
					t.Fatalf("refuse error must be *SchemaDriftError, got %T", err)
				}
				for _, needle := range []string{"schema_drift", "left untouched", EnvSchemaDriftRefuse} {
					if !strings.Contains(typed.Error(), needle) {
						t.Fatalf("halt message missing %q; got: %s", needle, typed.Error())
					}
				}
			} else if err != nil {
				t.Fatalf("want no error (shadow/healthy), got %v", err)
			}
		})
	}
}

// TestCheckSchemaDriftPropagatesReadError ensures a DB read failure is surfaced
// (fail-closed) rather than silently treated as in-sync.
func TestCheckSchemaDriftPropagatesReadError(t *testing.T) {
	sentinel := errors.New("db down")
	runner := &driftRunner{statePresent: true, scalarErr: sentinel}
	_, err := CheckSchemaDrift(context.Background(), runner)
	if !errors.Is(err, sentinel) {
		t.Fatalf("read error must propagate, got %v", err)
	}
	if errors.Is(err, ErrSchemaDrift) {
		t.Fatal("a read failure must not be misreported as schema drift")
	}
}

// TestSchemaDriftRefuseEnabledDefaultsOff pins the shadow-first default: with the
// env unset the gate is NOT in refuse mode. This is the RFC 0142 P3 safety
// contract — a boot-halt gate that lands to main must not auto-enforce on the next
// restart.
func TestSchemaDriftRefuseEnabledDefaultsOff(t *testing.T) {
	t.Setenv(EnvSchemaDriftRefuse, "")
	if SchemaDriftRefuseEnabled() {
		t.Fatal("STRIATUM_SCHEMA_DRIFT_REFUSE unset must default to shadow mode (refuse disabled)")
	}
	for _, on := range []string{"1", "true", "TRUE", "yes", "on"} {
		t.Setenv(EnvSchemaDriftRefuse, on)
		if !SchemaDriftRefuseEnabled() {
			t.Fatalf("STRIATUM_SCHEMA_DRIFT_REFUSE=%q must enable refuse mode", on)
		}
	}
	for _, off := range []string{"0", "false", "no", "off", "garbage", " "} {
		t.Setenv(EnvSchemaDriftRefuse, off)
		if SchemaDriftRefuseEnabled() {
			t.Fatalf("STRIATUM_SCHEMA_DRIFT_REFUSE=%q must NOT enable refuse mode", off)
		}
	}
}

// TestLiveFingerprintAbsentTable returns "" (not an error) when the schema_state
// table is not present — a pre-P3 database. "" is treated as not-yet-recorded.
func TestLiveFingerprintAbsentTable(t *testing.T) {
	runner := &driftRunner{statePresent: false}
	live, err := LiveFingerprint(context.Background(), runner)
	if err != nil {
		t.Fatalf("LiveFingerprint on absent table: %v", err)
	}
	if live != "" {
		t.Fatalf("LiveFingerprint on absent table = %q; want empty", live)
	}
	if runner.wroteAnything() {
		t.Fatalf("LiveFingerprint must be read-only: %v", runner.queries)
	}
}
