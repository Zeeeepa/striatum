package db_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// v3fields holds the 15 audit fields with Go-native types so each test case can
// drive both V3RowHash (Go) and striatumd.audit_v3_row_hash (SQL) from one
// source. Nullable fields use any (nil = SQL NULL).
type v3fields struct {
	ts                time.Time
	schemaVersion     int
	hashFormatVersion int
	daemonVersion     string
	clientID          any
	repositoryID      any
	method            string
	decision          string
	denialReason      any
	transport         string
	requestID         any
	exitCode          any
	paramsSHA256      string
	previousHash      any
	segmentID         int64
}

func (f v3fields) goRow() map[string]any {
	return map[string]any{
		"ts":                  f.ts.UTC().Truncate(time.Second).Format("2006-01-02T15:04:05Z"),
		"schema_version":      f.schemaVersion,
		"hash_format_version": f.hashFormatVersion,
		"daemon_version":      f.daemonVersion,
		"client_id":           f.clientID,
		"repository_id":       f.repositoryID,
		"method":              f.method,
		"decision":            f.decision,
		"denial_reason":       f.denialReason,
		"transport":           f.transport,
		"request_id":          f.requestID,
		"exit_code":           f.exitCode,
		"params_sha256":       f.paramsSHA256,
		"previous_hash":       f.previousHash,
		"segment_id":          f.segmentID,
	}
}

func (f v3fields) sqlHash(t *testing.T, ctx context.Context, runner db.Runner) string {
	t.Helper()
	v, err := runner.QueryScalar(ctx, `SELECT striatumd.audit_v3_row_hash(
		$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		f.ts.UTC().Truncate(time.Second), f.schemaVersion, f.hashFormatVersion,
		f.daemonVersion, f.clientID, f.repositoryID, f.method, f.decision,
		f.denialReason, f.transport, f.requestID, f.exitCode, f.paramsSHA256,
		f.previousHash, f.segmentID)
	if err != nil {
		t.Fatalf("sql audit_v3_row_hash: %v", err)
	}
	return v
}

// TestAuditV3HashParity is T-HASH-PARITY (RFC 0110 §5.1): the Go V3RowHash and
// the in-database striatumd.audit_v3_row_hash produce byte-identical hashes
// across ASCII, multibyte UTF-8, HTML/JSON-hazard characters, the JSON line/
// paragraph separators, empty strings, nulls, integers, and segment edges.
func TestAuditV3HashParity(t *testing.T) {
	pool := pgtest.Pool(t)
	ctx := context.Background()
	if _, _, err := db.ApplyOwnerBundles(ctx, pool.Runner, "test"); err != nil {
		t.Fatalf("apply owner bundle: %v", err)
	}

	base := time.Date(2026, 6, 4, 12, 34, 56, 0, time.UTC)
	cases := map[string]v3fields{
		"ascii": {ts: base, schemaVersion: 1, hashFormatVersion: 3, daemonVersion: "v2.16.0",
			clientID: "client-1", repositoryID: "repo_abc", method: "work.complete", decision: "allowed",
			denialReason: nil, transport: "rpc", requestID: "req-1", exitCode: nil,
			paramsSHA256: "abc123", previousHash: nil, segmentID: 1},
		"multibyte": {ts: base, schemaVersion: 1, hashFormatVersion: 3, daemonVersion: "héllo-日本語",
			clientID: "клиент", repositoryID: "rëpo", method: "work.ack", decision: "allowed",
			denialReason: nil, transport: "mcp", requestID: "req-✓", exitCode: 10,
			paramsSHA256: "déadbeef", previousHash: "prev-héx", segmentID: 7},
		"html_json_hazards": {ts: base, schemaVersion: 1, hashFormatVersion: 3, daemonVersion: "<a>&\"'</a>",
			clientID: "a<b>c", repositoryID: "r&d", method: "x>y", decision: "denied",
			denialReason: "needs <escape> & \"quotes\"", transport: "rpc", requestID: "q&a", exitCode: 10,
			paramsSHA256: "00", previousHash: "<prev>", segmentID: 2},
		"unicode_separators": {ts: base, schemaVersion: 1, hashFormatVersion: 3, daemonVersion: "line para sep",
			clientID: "a b", repositoryID: nil, method: "m", decision: "allowed",
			denialReason: nil, transport: "rpc", requestID: nil, exitCode: nil,
			paramsSHA256: "ff", previousHash: nil, segmentID: 3},
		"empty_and_nulls": {ts: base, schemaVersion: 1, hashFormatVersion: 3, daemonVersion: "",
			clientID: "", repositoryID: nil, method: "", decision: "allowed",
			denialReason: nil, transport: "", requestID: "", exitCode: nil,
			paramsSHA256: "", previousHash: nil, segmentID: 0},
		"big_integers": {ts: base, schemaVersion: 1, hashFormatVersion: 3, daemonVersion: "v",
			clientID: nil, repositoryID: nil, method: "m", decision: "allowed",
			denialReason: nil, transport: "rpc", requestID: nil, exitCode: 255,
			paramsSHA256: "x", previousHash: nil, segmentID: 9223372036854775807},
		"ts_midnight": {ts: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), schemaVersion: 1, hashFormatVersion: 3,
			daemonVersion: "v", clientID: nil, repositoryID: nil, method: "m", decision: "allowed",
			denialReason: nil, transport: "rpc", requestID: nil, exitCode: nil,
			paramsSHA256: "x", previousHash: nil, segmentID: 1},
	}
	for name, f := range cases {
		t.Run(name, func(t *testing.T) {
			goHash, err := db.V3RowHash(f.goRow())
			if err != nil {
				t.Fatalf("V3RowHash: %v", err)
			}
			sqlHash := f.sqlHash(t, ctx, pool.Runner)
			if goHash != sqlHash {
				t.Fatalf("hash mismatch:\n  go  = %s\n  sql = %s", goHash, sqlHash)
			}
		})
	}
}

// TestAuditV3HashTimezoneIndependent is T-TS (RFC 0110 §5.1): the SQL hash is
// independent of the session TimeZone (the builder pins AT TIME ZONE 'UTC') and
// equals the Go hash under every zone.
func TestAuditV3HashTimezoneIndependent(t *testing.T) {
	pool := pgtest.Pool(t)
	ctx := context.Background()
	if _, _, err := db.ApplyOwnerBundles(ctx, pool.Runner, "test"); err != nil {
		t.Fatalf("apply owner bundle: %v", err)
	}
	f := v3fields{ts: time.Date(2026, 6, 4, 23, 59, 0, 0, time.UTC), schemaVersion: 1, hashFormatVersion: 3,
		daemonVersion: "v2.16.0", clientID: "c", repositoryID: "r", method: "m", decision: "allowed",
		denialReason: nil, transport: "rpc", requestID: "q", exitCode: nil, paramsSHA256: "p",
		previousHash: nil, segmentID: 4}
	goHash, err := db.V3RowHash(f.goRow())
	if err != nil {
		t.Fatalf("V3RowHash: %v", err)
	}
	for _, tz := range []string{"UTC", "EST", "Asia/Kolkata", "Australia/Lord_Howe"} {
		if err := pool.Runner.Exec(ctx, fmt.Sprintf("SET TimeZone = %s", quoteLiteral(tz))); err != nil {
			t.Fatalf("set timezone %s: %v", tz, err)
		}
		if got := f.sqlHash(t, ctx, pool.Runner); got != goHash {
			t.Fatalf("timezone %s changed the hash: sql=%s go=%s", tz, got, goHash)
		}
	}
}

func quoteLiteral(s string) string { return "'" + s + "'" }

// TestVerifyRowsMixed is T-VERIFY-MIXED (RFC 0110 §5.1): VerifyRows dispatches
// per hash_format_version, holds continuity across a v2->v3 boundary, fails on
// an unknown format, and fails on a per-format tamper.
func TestVerifyRowsMixed(t *testing.T) {
	mkRow := func(id int, format int, prev any, method string) map[string]any {
		row := map[string]any{
			"audit_id": id, "ts": "2026-06-04T00:00:0" + fmt.Sprint(id) + "Z",
			"schema_version": 1, "hash_format_version": format, "daemon_version": "v",
			"client_id": nil, "repository_id": nil, "method": method, "decision": "allowed",
			"denial_reason": nil, "transport": "rpc", "request_id": nil, "exit_code": nil,
			"params_sha256": "p", "previous_hash": prev, "segment_id": int64(1),
		}
		var h string
		var err error
		if format == 2 {
			h, err = db.V2RowHash(row)
		} else {
			h, err = db.V3RowHash(row)
		}
		if err != nil {
			t.Fatalf("hash row %d: %v", id, err)
		}
		row["row_hash"] = h
		return row
	}

	// v2-only valid chain.
	r1 := mkRow(1, 2, nil, "a")
	r2 := mkRow(2, 2, r1["row_hash"], "b")
	if probs := db.VerifyRows([]map[string]any{r1, r2}); len(probs) != 0 {
		t.Fatalf("valid v2 chain flagged: %v", probs)
	}

	// v3-only valid chain.
	r3 := mkRow(1, 3, nil, "a")
	r4 := mkRow(2, 3, r3["row_hash"], "b")
	if probs := db.VerifyRows([]map[string]any{r3, r4}); len(probs) != 0 {
		t.Fatalf("valid v3 chain flagged: %v", probs)
	}

	// mixed v2 -> v3 with continuity.
	m1 := mkRow(1, 2, nil, "a")
	m2 := mkRow(2, 3, m1["row_hash"], "b")
	if probs := db.VerifyRows([]map[string]any{m1, m2}); len(probs) != 0 {
		t.Fatalf("valid mixed chain flagged: %v", probs)
	}

	// unknown format -> failure (no silent v2 fallback).
	u1 := mkRow(1, 3, nil, "a")
	u2 := map[string]any{"audit_id": 2, "ts": "2026-06-04T00:00:02Z", "schema_version": 1,
		"hash_format_version": 4, "daemon_version": "v", "method": "b", "decision": "allowed",
		"transport": "rpc", "params_sha256": "p", "previous_hash": u1["row_hash"],
		"segment_id": int64(1), "row_hash": "deadbeef"}
	if probs := db.VerifyRows([]map[string]any{u1, u2}); len(probs) == 0 {
		t.Fatal("unknown hash_format_version was not flagged")
	}

	// per-format tamper: a v3 row with a corrupted row_hash.
	tp1 := mkRow(1, 3, nil, "a")
	tp1["row_hash"] = "0000000000000000000000000000000000000000000000000000000000000000"
	if probs := db.VerifyRows([]map[string]any{tp1}); len(probs) == 0 {
		t.Fatal("tampered v3 row_hash was not flagged")
	}
}
