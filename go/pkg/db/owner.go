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
const LatestOwnerBundleVersion = 1

//go:embed sql/owner/*.sql
var ownerBundleFS embed.FS

var ownerBundleLabels = map[int]string{
	1: "authority schema + v3 hash + phase 0 audit_only (RFC 0110 N+1)",
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
