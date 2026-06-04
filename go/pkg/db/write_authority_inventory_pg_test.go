package db_test

import (
	"context"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// TestWriteAuthorityInventoryComplete is the PX3-006 guard (RFC 0110 §13): every
// daemon-owned table — runtime migrations plus the owner bundle — carries a
// write-authority classification, so a new table cannot silently bypass the L1
// model. Generated against information_schema so nothing is missed.
func TestWriteAuthorityInventoryComplete(t *testing.T) {
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
		if _, ok := db.ClassifyTable(name); !ok {
			unclassified = append(unclassified, name)
		}
	}
	if err := rs.Err(); err != nil {
		t.Fatalf("rows err: %v", err)
	}
	if count == 0 {
		t.Fatal("no striatumd tables found; the inventory guard would be vacuous")
	}
	if len(unclassified) > 0 {
		t.Fatalf("striatumd tables missing a write-authority inventory row (RFC 0110 §13): %v\n"+
			"add each to writeAuthorityInventory in write_authority_inventory.go", unclassified)
	}
}
