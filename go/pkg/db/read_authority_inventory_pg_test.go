package db_test

import (
	"context"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// TestReadAuthorityInventoryComplete is the #164 guard: every daemon-owned
// table has an explicit read classification, so the current broad-runtime-SELECT
// posture cannot grow silently while the least-privilege split is still open.
func TestReadAuthorityInventoryComplete(t *testing.T) {
	pool := pgtest.Pool(t)
	ctx := context.Background()
	if _, _, err := db.ApplyOwnerBundles(ctx, pool.Runner, "test"); err != nil {
		t.Fatalf("apply owner bundle: %v", err)
	}

	rs, err := pool.RawPool.Query(ctx,
		`SELECT table_name FROM information_schema.tables
		  WHERE table_schema = 'striatumd' AND table_type = 'BASE TABLE'
		  ORDER BY table_name`)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	defer rs.Close()

	var unclassified []string
	count := 0
	for rs.Next() {
		var name string
		if err := rs.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		count++
		if _, ok := db.ClassifyReadTable(name); !ok {
			unclassified = append(unclassified, name)
		}
	}
	if err := rs.Err(); err != nil {
		t.Fatalf("rows err: %v", err)
	}
	if count == 0 {
		t.Fatal("no striatumd tables found; the read inventory guard would be vacuous")
	}
	if len(unclassified) > 0 {
		t.Fatalf("striatumd tables missing a read-authority inventory row (#164): %v\n"+
			"add each to readAuthorityInventory in read_authority_inventory.go", unclassified)
	}
}

// TestReadDeniedTablesHaveNoRuntimeSelect keeps the authority registry and
// other owner-only read surfaces denied to the runtime role while #164 works on
// reducing the still-broad selectable surface.
func TestReadDeniedTablesHaveNoRuntimeSelect(t *testing.T) {
	pool := pgtest.Pool(t)
	ctx := context.Background()
	if _, _, err := db.ApplyOwnerBundles(ctx, pool.Runner, "test"); err != nil {
		t.Fatalf("apply owner bundle: %v", err)
	}

	for table, class := range db.ReadAuthorityInventory() {
		if class != db.ReadClassRuntimeDenied {
			continue
		}
		if scalar(t, ctx, pool.Runner,
			"SELECT has_table_privilege('striatumd_rw', 'striatumd.'||$1, 'SELECT')::text", table) != "false" {
			t.Fatalf("striatumd_rw can SELECT %s; read inventory class is %s", table, class)
		}
	}
}

// TestReadDeniedColumnsHaveNoRuntimeSelect pins the RFC 0113 R1 column-level
// reduction: client token hashes/salts are no longer directly selectable by the
// runtime role after owner bundle 0005, even though non-secret client metadata
// remains readable while the broader #164 split continues.
func TestReadDeniedColumnsHaveNoRuntimeSelect(t *testing.T) {
	pool := pgtest.Pool(t)
	ctx := context.Background()
	if _, _, err := db.ApplyOwnerBundles(ctx, pool.Runner, "test"); err != nil {
		t.Fatalf("apply owner bundle: %v", err)
	}

	for table, columns := range db.RuntimeDeniedReadColumns() {
		for _, column := range columns {
			if scalar(t, ctx, pool.Runner,
				"SELECT has_column_privilege('striatumd_rw', 'striatumd.'||$1, $2, 'SELECT')::text", table, column) != "false" {
				t.Fatalf("striatumd_rw can SELECT %s.%s; read inventory marks it denied", table, column)
			}
		}
	}
	if scalar(t, ctx, pool.Runner,
		"SELECT has_column_privilege('striatumd_rw', 'striatumd.clients', 'client_id', 'SELECT')::text") != "true" {
		t.Fatal("striatumd_rw lost non-secret clients.client_id SELECT while only token secret columns should be denied")
	}
}
