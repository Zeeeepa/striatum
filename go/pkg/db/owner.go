package db

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// LatestOwnerBundleVersion is the highest owner-DDL bundle the binary ships.
// Owner bundles (RFC 0110 §8.1) carry owner-only DDL — the authority registry,
// SECURITY DEFINER write functions, capability stamps, and the phased DML
// revokes — that the runtime role cannot perform. They are applied OUT-OF-BAND
// as the database owner via `striatum daemon owner-ddl apply`, never through the
// runtime-role ApplyMigrations path (RFC 0079 §5).
const LatestOwnerBundleVersion = 4

//go:embed sql/owner/*.sql
var ownerBundleFS embed.FS

var ownerBundleLabels = map[int]string{
	1: "authority schema + v3 hash + phase 0 audit_only (RFC 0110 N+1)",
	2: "runtime read grant on schema_authority for capability parity (RFC 0110 N+1)",
	3: "phase 1 audit_artifacts: append_artifact_row SD fn + artifacts INSERT revoke (RFC 0110 §7)",
	4: "phase 2 full: append_event_row SD fn (in-DB v3 hash + transcript exclusion) + events INSERT revoke (RFC 0110 §7)",
}

// OwnerBundle is one versioned owner-DDL bundle file.
type OwnerBundle struct {
	Version int
	Label   string
	Path    string
	SQL     string
}

// SHA256 is the content hash recorded in owner_bundle_meta on apply.
func (b OwnerBundle) SHA256() string {
	sum := sha256.Sum256([]byte(b.SQL))
	return hex.EncodeToString(sum[:])
}

// OwnerBundles returns the embedded owner bundles in ascending version order.
func OwnerBundles() ([]OwnerBundle, error) {
	entries, err := ownerBundleFS.ReadDir("sql/owner")
	if err != nil {
		return nil, err
	}
	var bundles []OwnerBundle
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version, err := strconv.Atoi(strings.SplitN(entry.Name(), "_", 2)[0])
		if err != nil {
			return nil, fmt.Errorf("owner bundle %s has no leading version: %w", entry.Name(), err)
		}
		body, err := ownerBundleFS.ReadFile("sql/owner/" + entry.Name())
		if err != nil {
			return nil, err
		}
		bundles = append(bundles, OwnerBundle{
			Version: version,
			Label:   ownerBundleLabels[version],
			Path:    "sql/owner/" + entry.Name(),
			SQL:     string(body),
		})
	}
	sort.Slice(bundles, func(i, j int) bool { return bundles[i].Version < bundles[j].Version })
	return bundles, nil
}

// OwnerBundleVersion returns the highest owner bundle version applied to the
// database, or 0 when no bundle (and hence no owner_bundle_meta table) exists.
func OwnerBundleVersion(ctx context.Context, runner Runner) (int, error) {
	present, err := runner.QueryScalar(ctx,
		"SELECT (to_regclass('striatumd.owner_bundle_meta') IS NOT NULL)::text")
	if err != nil {
		return 0, err
	}
	if present != "true" {
		return 0, nil
	}
	value, err := runner.QueryScalar(ctx,
		"SELECT COALESCE(MAX(version), 0)::text FROM striatumd.owner_bundle_meta")
	if err != nil {
		return 0, err
	}
	version, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, err
	}
	return version, nil
}

// ApplyOwnerBundles applies every owner bundle newer than the recorded version,
// as the owner role behind runner. Each version applies in a single transaction
// that stamps owner_bundle_meta last, so a partially-applied bundle cannot
// persist (atomic per version); re-applying a stamped version is a no-op. It
// returns the versions applied this call and the resulting version.
func ApplyOwnerBundles(ctx context.Context, runner Runner, daemonVersion string) ([]int, int, error) {
	if daemonVersion == "" {
		daemonVersion = "dev"
	}
	bundles, err := OwnerBundles()
	if err != nil {
		return nil, 0, err
	}
	current, err := OwnerBundleVersion(ctx, runner)
	if err != nil {
		return nil, 0, err
	}
	var applied []int
	for _, bundle := range bundles {
		if bundle.Version <= current {
			continue
		}
		if err := applyOneOwnerBundle(ctx, runner, bundle, daemonVersion); err != nil {
			return applied, current, fmt.Errorf("apply owner bundle %d (%s): %w", bundle.Version, bundle.Label, err)
		}
		applied = append(applied, bundle.Version)
		current = bundle.Version
	}
	return applied, current, nil
}

// capabilityProtectedTable maps each SD-append capability stamp to the table
// whose direct runtime-role INSERT that phase revokes (RFC 0110 §7). The
// protected set is derived from the stamps rather than a static list so a
// deployment revokes exactly the surfaces its applied owner bundles have closed
// — never a surface whose bundle is unapplied (which would break that surface's
// writes for a daemon that has not adopted the phase).
var capabilityProtectedTable = map[string]string{
	"audit_sd_append":    "audit_log",
	"artifact_sd_append": "artifacts",
	"event_sd_append":    "events",
}

// ReassertWriteRevokes re-applies the protected-surface INSERT revokes for every
// phase whose capability the owner bundle has stamped (RFC 0110 §6,
// C-GRANT-DRIFT). Any privilege-granting step (migration helper, doctor
// repair-grants) must call this afterwards so a stray GRANT cannot quietly
// reopen a closed surface. Run as the owner; a no-op when no authority schema is
// present.
func ReassertWriteRevokes(ctx context.Context, runner Runner) error {
	stamped, present, err := readStampedCapabilities(ctx, runner)
	if err != nil {
		return err
	}
	if !present {
		return nil
	}
	for _, capability := range stamped {
		table, ok := capabilityProtectedTable[capability]
		if !ok {
			continue
		}
		if err := runner.Exec(ctx, fmt.Sprintf(
			"REVOKE INSERT ON striatumd.%s FROM striatumd_rw", table)); err != nil {
			return fmt.Errorf("reassert revoke on %s: %w", table, err)
		}
	}
	return nil
}

func applyOneOwnerBundle(ctx context.Context, runner Runner, bundle OwnerBundle, daemonVersion string) error {
	tx, err := runner.BeginTx(ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(context.Background())
		}
	}()
	if err := tx.Exec(ctx, bundle.SQL); err != nil {
		return err
	}
	// The bundle creates owner_bundle_meta (IF NOT EXISTS); the stamp is the
	// last write in the same transaction, so the marker and the objects commit
	// together or not at all.
	if err := tx.Exec(ctx,
		`INSERT INTO striatumd.owner_bundle_meta(version, label, sha256, daemon_version)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (version) DO NOTHING`,
		bundle.Version, bundle.Label, bundle.SHA256(), daemonVersion,
	); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	committed = true
	return nil
}
