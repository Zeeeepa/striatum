package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// SupportedAuthorityCapabilities are the daemon-authority capabilities this
// binary understands. The owner bundle stamps a subset (RFC 0110 §8.2); startup
// parity fails closed if the schema stamps a capability this binary cannot
// support (old-binary / authority-bearing-schema).
func SupportedAuthorityCapabilities() []string {
	return []string{"audit_sd_append"}
}

// RequiredAuthorityCapabilities are capabilities the binary requires the schema
// to stamp. Empty in release N+1 slice 1: the owner bundle is opt-in and the
// daemon does not yet depend on the SD write path, so a binary with no bundle
// applied runs inert — which is exactly what makes the old-binary check real
// once any binary >= this one meets an authority-bearing schema.
func RequiredAuthorityCapabilities() []string {
	return nil
}

// checkCapabilityParity enforces RFC 0110 §8.2 deploy capability parity in both
// directions, but only once an authority-bearing schema exists. Until then
// (release N: no owner bundle has stamped any capability) it is inert and
// passes — which is exactly what makes the old-binary check real later: every
// binary that can ever meet an authority-bearing schema is >= N and so carries
// this checker.
//
//   - new-binary / old-schema: every capability the binary requires must be
//     stamped, or boot fails closed naming the missing capability.
//   - old-binary / authority-schema: every stamped capability must be one the
//     binary supports, or boot fails closed naming the unknown capability.
func checkCapabilityParity(required, supported, stamped []string, authoritySchema bool) error {
	if !authoritySchema {
		return nil
	}
	stampedSet := toSet(stamped)
	supportedSet := toSet(supported)
	for _, req := range required {
		if !stampedSet[req] {
			return fmt.Errorf("daemon_pg_schema_precondition: binary requires capability %q that the owner bundle has not stamped (binary newer than schema; apply the owner bundle)", req)
		}
	}
	for _, stamp := range stamped {
		if !supportedSet[stamp] {
			return fmt.Errorf("daemon_pg_schema_precondition: schema stamps capability %q this binary does not support (binary older than schema; upgrade the binary)", stamp)
		}
	}
	return nil
}

func toSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}

// VerifyCapabilityParity checks RFC 0110 §8.2 deploy capability parity against
// the live schema before the daemon serves mutations. In release N no authority
// schema exists (the schema_authority marker table is absent), so it is an inert
// pass-through; release N+1's owner bundle introduces the markers this enforces.
func VerifyCapabilityParity(ctx context.Context, runner Runner, required, supported []string) error {
	stamped, present, err := readStampedCapabilities(ctx, runner)
	if err != nil {
		return err
	}
	return checkCapabilityParity(required, supported, stamped, present)
}

// readStampedCapabilities returns the capabilities stamped by the owner bundle
// and whether an authority-bearing schema is present at all. In release N the
// schema_authority table does not exist, so present is false (inert).
func readStampedCapabilities(ctx context.Context, runner Runner) ([]string, bool, error) {
	// The ::text cast forces SQL-side 'true'/'false'; a bare boolean scans to
	// the Postgres text repr ('t'/'f') under simple protocol, which would make
	// the present case wrongly read as absent.
	exists, err := runner.QueryScalar(ctx,
		"SELECT (to_regclass('striatumd.schema_authority') IS NOT NULL)::text")
	if err != nil {
		return nil, false, err
	}
	if exists != "true" {
		return nil, false, nil
	}
	rows, err := queryStrings(ctx, runner,
		"SELECT capability FROM striatumd.schema_authority WHERE requires_daemon_auth = true")
	if err != nil {
		return nil, false, err
	}
	return rows, true, nil
}

func queryStrings(ctx context.Context, runner Runner, sql string) ([]string, error) {
	type rowsQuerier interface {
		Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	}
	q, ok := runner.(rowsQuerier)
	if !ok {
		// QueryScalar-only runner: no rows (used only by the inert path in
		// release N, where the schema_authority table is absent anyway).
		return nil, nil
	}
	rows, err := q.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
