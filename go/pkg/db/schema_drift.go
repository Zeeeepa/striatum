package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// EnvSchemaDriftRefuse is the opt-in environment variable that promotes the RFC
// 0142 Layer 3 part 1 (P3) schema-fingerprint drift gate from SHADOW mode (the
// default: log a loud warning and continue serving) to HARD REFUSE-TO-SERVE (on
// drift, return the typed schema_drift halt and exit via the same
// non-restartable path the Layer 2 watermark interlock uses).
//
// Shadow-first is deliberate and matches the codebase convention for a risky new
// boot gate (cf. STRIATUM_AUTO_SPAWN_SCHEDULER, default OFF): a boot-halt gate
// that lands to main auto-applies on the NEXT daemon restart, so if the
// fingerprint logic has any false-positive it would convert a healthy deploy into
// an outage with no operator action. Defaulting OFF means P3 ships the loud,
// test-covered DETECTION first; an operator turns on enforcement per deployment
// only once the fingerprint contract has been observed clean in shadow.
const EnvSchemaDriftRefuse = "STRIATUM_SCHEMA_DRIFT_REFUSE"

// schemaStateSingletonID is the constant primary key of the single schema_state
// row (the 0043 migration enforces id = 'singleton').
const schemaStateSingletonID = "singleton"

// ErrSchemaDrift is the sentinel for the RFC 0142 Layer 3 part 1 drift condition:
// the live recorded schema fingerprint diverges from the fingerprint this binary
// expects. It is wrapped by *SchemaDriftError so callers can branch with
// errors.Is / errors.As while still printing the operator-facing detail.
var ErrSchemaDrift = errors.New("schema_drift")

// SchemaDriftError carries the expected-vs-live schema fingerprints for the RFC
// 0142 Layer 3 part 1 drift gate. It is returned ONLY in refuse mode
// (STRIATUM_SCHEMA_DRIFT_REFUSE truthy); in the default shadow mode the gate logs
// a warning instead of returning this. It wraps ErrSchemaDrift.
type SchemaDriftError struct {
	Expected string
	Live     string
}

// Error renders the actionable drift message: the diverging fingerprints and the
// reconcile pointer. (The precise per-step missing/extra enumeration is P4's
// deploy-plan concern; P3 reports the fingerprint divergence and that the gate
// refused because enforcement is enabled.)
func (e *SchemaDriftError) Error() string {
	live := e.Live
	if live == "" {
		live = "<none recorded>"
	}
	return fmt.Sprintf(
		"schema_drift: live schema fingerprint %s diverges from the %s this binary expects; "+
			"the applied schema does not match the runtime-migration + owner-bundle set this binary embeds "+
			"(RFC 0142 Layer 3). Reconcile the database to the binary (apply pending migrations/owner bundles, "+
			"or roll the binary back to the matching frontier) before serving. The database was left untouched. "+
			"This refusal is gated on %s=1; unset it to run in shadow mode (log + continue).",
		live, e.Expected, EnvSchemaDriftRefuse)
}

// Unwrap lets errors.Is(err, ErrSchemaDrift) match a *SchemaDriftError.
func (e *SchemaDriftError) Unwrap() error { return ErrSchemaDrift }

// ExpectedFingerprint is the deterministic sha256-hex digest of the ordered
// runtime-migration SHA set PLUS the ordered owner-bundle SHA set the binary
// embeds (RFC 0142 Layer 3 part 1). It is the "schema fingerprint contract": two
// processes built from the same source compute the same value, and ANY change to
// the embedded migration or owner-bundle byte content (including adding/removing a
// step) changes it.
//
// Composition is order-stable and content-addressed: we sort both sets by their
// versioned filename, then hash a canonical newline-delimited transcript of
// `filename=sha256` lines, each section preceded by a fixed tag. Sorting by the
// versioned filename (not map-iteration order) is what makes the digest stable
// across processes; tagging the two sections keeps a migration and an owner bundle
// that happened to share a filename/hash from colliding across the boundary.
func ExpectedFingerprint() (string, error) {
	migSet, err := MigrationSHASet()
	if err != nil {
		return "", err
	}
	bundles, err := OwnerBundles()
	if err != nil {
		return "", err
	}
	bundleSet := map[string]string{}
	for _, b := range bundles {
		// Key by the leading version (zero-padded) so the ordering is by applied
		// order, identical to how OwnerBundles() sorts. Using the version keeps the
		// transcript independent of the embed FS filename shape.
		bundleSet[fmt.Sprintf("%04d", b.Version)] = b.SHA256()
	}
	return composeFingerprint(migSet, bundleSet), nil
}

// composeFingerprint builds the canonical, order-stable transcript and hashes it.
// migSet is keyed by migration filename, bundleSet by zero-padded bundle version;
// both are sorted by key so the digest is independent of map-iteration order.
func composeFingerprint(migSet, bundleSet map[string]string) string {
	var b strings.Builder
	b.WriteString("striatum.schema_fingerprint.v1\n")

	b.WriteString("migrations:\n")
	migKeys := make([]string, 0, len(migSet))
	for k := range migSet {
		migKeys = append(migKeys, k)
	}
	sort.Strings(migKeys)
	for _, k := range migKeys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(migSet[k])
		b.WriteByte('\n')
	}

	b.WriteString("owner_bundles:\n")
	bundleKeys := make([]string, 0, len(bundleSet))
	for k := range bundleSet {
		bundleKeys = append(bundleKeys, k)
	}
	sort.Strings(bundleKeys)
	for _, k := range bundleKeys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(bundleSet[k])
		b.WriteByte('\n')
	}

	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// LiveFingerprint reads the recorded schema fingerprint from the schema_state
// singleton row, returning "" when no row (or no schema_state table) is present —
// e.g. a database migrated by a pre-P3 binary, or a fresh database before the
// first self-record. A "" Live is treated as NO drift (not-yet-recorded, not
// divergent) by CheckSchemaDrift, so an older database upgrades cleanly: the first
// successful ApplyMigrations under a P3 binary records the row.
func LiveFingerprint(ctx context.Context, runner Runner) (string, error) {
	present, err := runner.QueryScalar(ctx,
		"SELECT (to_regclass('striatumd.schema_state') IS NOT NULL)::text")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(present) != "true" {
		return "", nil
	}
	value, err := runner.QueryScalar(ctx,
		"SELECT COALESCE(fingerprint, '') FROM striatumd.schema_state WHERE id = $1",
		schemaStateSingletonID)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

// RecordSchemaFingerprint UPSERTs the current ExpectedFingerprint into the
// schema_state singleton row. It is called AFTER ApplyMigrations succeeds, so a
// correctly-migrated daemon always reads Live == Expected on the next boot and the
// drift gate never false-halts. It writes only the singleton row.
//
// It no-ops silently if the schema_state table is not yet present (a partial
// migration ordering where 0043 has not applied), so it never aborts a successful
// migrate; the next clean boot records it.
func RecordSchemaFingerprint(ctx context.Context, runner Runner, daemonVersion string) error {
	fp, err := ExpectedFingerprint()
	if err != nil {
		return err
	}
	present, err := runner.QueryScalar(ctx,
		"SELECT (to_regclass('striatumd.schema_state') IS NOT NULL)::text")
	if err != nil {
		return err
	}
	if strings.TrimSpace(present) != "true" {
		return nil
	}
	if daemonVersion == "" {
		daemonVersion = "dev"
	}
	return runner.Exec(ctx, `
INSERT INTO striatumd.schema_state (id, fingerprint, daemon_version, applied_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (id) DO UPDATE
   SET fingerprint = EXCLUDED.fingerprint,
       daemon_version = EXCLUDED.daemon_version,
       applied_at = EXCLUDED.applied_at`,
		schemaStateSingletonID, fp, daemonVersion)
}

// SchemaDriftRefuseEnabled reports whether the operator opted the drift gate into
// HARD refuse-to-serve via STRIATUM_SCHEMA_DRIFT_REFUSE. Default (unset/empty) is
// SHADOW mode (log + continue). Accepts the usual truthy spellings.
func SchemaDriftRefuseEnabled() bool {
	return envTruthy(os.Getenv(EnvSchemaDriftRefuse))
}

func envTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "t", "yes", "y", "on":
		return true
	}
	if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
		return b
	}
	return false
}

// SchemaDriftDecision is the pure policy result of comparing the expected and live
// fingerprints, independent of the env flag — so the doctor surface can report
// drift regardless of whether enforcement is on.
type SchemaDriftDecision struct {
	Expected string
	Live     string
	// Drift is true when a fingerprint is recorded AND it diverges from expected.
	// A "" Live (not yet recorded / pre-P3 database) is NOT drift.
	Drift bool
}

// EvaluateSchemaDrift is the pure comparison (no DB, no env): given expected and
// live, decide whether the live schema has drifted. An empty Live is "not yet
// recorded" — never drift — so an older database (or a fresh one before the first
// self-record) upgrades cleanly. Equal fingerprints are in-sync.
func EvaluateSchemaDrift(expected, live string) SchemaDriftDecision {
	d := SchemaDriftDecision{Expected: expected, Live: live}
	if live == "" {
		return d // not recorded yet → not drift
	}
	d.Drift = live != expected
	return d
}

// CheckSchemaDrift is the boot-guard for the RFC 0142 Layer 3 part 1 drift gate.
// It is a PURE READ (it mutates nothing). It computes ExpectedFingerprint, reads
// LiveFingerprint, and:
//
//   - No drift (live == expected, or live not yet recorded): returns (false, nil).
//     Boot proceeds exactly as before.
//   - Drift, refuse DISABLED (default SHADOW mode): returns (true, nil). The
//     caller logs a loud warning and CONTINUES serving. This is the default so a
//     false-positive cannot auto-cause an outage on a bare restart.
//   - Drift, refuse ENABLED (STRIATUM_SCHEMA_DRIFT_REFUSE truthy): returns
//     (true, *SchemaDriftError). The caller halts (refuse-to-serve) and exits via
//     the same non-restartable path the Layer 2 watermark interlock uses.
//
// It returns the drift boolean even on the nil-error shadow path so the caller can
// log the warning; the *SchemaDriftError (when refuse is on) carries the detail.
func CheckSchemaDrift(ctx context.Context, runner Runner) (bool, error) {
	expected, err := ExpectedFingerprint()
	if err != nil {
		return false, err
	}
	live, err := LiveFingerprint(ctx, runner)
	if err != nil {
		return false, err
	}
	decision := EvaluateSchemaDrift(expected, live)
	if !decision.Drift {
		return false, nil
	}
	if !SchemaDriftRefuseEnabled() {
		// SHADOW mode: surface the drift to the caller (which logs a loud warning)
		// but do NOT block boot.
		return true, nil
	}
	// REFUSE mode: hard halt with the typed error.
	return true, &SchemaDriftError{Expected: expected, Live: live}
}
