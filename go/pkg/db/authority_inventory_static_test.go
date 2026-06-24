package db

import (
	"sort"
	"strings"
	"testing"
)

func TestReadAuthorityInventoryCoversEmbeddedTablesWithoutPostgres(t *testing.T) {
	assertAuthorityInventoryCoversEmbeddedTables(t, "read", ReadAuthorityInventory())
}

func TestWriteAuthorityInventoryCoversEmbeddedTablesWithoutPostgres(t *testing.T) {
	assertAuthorityInventoryCoversEmbeddedTables(t, "write", WriteAuthorityInventory())
}

func assertAuthorityInventoryCoversEmbeddedTables[T ~string](t *testing.T, name string, inventory map[string]T) {
	t.Helper()
	tables := embeddedStriatumdTables(t)
	if len(tables) == 0 {
		t.Fatal("embedded SQL created no striatumd tables; authority inventory guard would be vacuous")
	}
	var missing []string
	for table := range tables {
		if _, ok := inventory[table]; !ok {
			missing = append(missing, table)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("embedded striatumd tables missing %s-authority inventory rows: %v", name, missing)
	}
}

func embeddedStriatumdTables(t *testing.T) map[string]bool {
	t.Helper()
	tables := map[string]bool{}
	migrations, err := Migrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	for _, migration := range migrations {
		recordCreatedTables(tables, migration.SQL)
	}
	bundles, err := OwnerBundles()
	if err != nil {
		t.Fatalf("load owner bundles: %v", err)
	}
	for _, bundle := range bundles {
		recordCreatedTables(tables, bundle.SQL)
	}
	return tables
}

func recordCreatedTables(tables map[string]bool, sql string) {
	stripped := sqlLineCommentPattern.ReplaceAllString(sql, "")
	for _, match := range runtimeMigrationCreateTablePattern.FindAllStringSubmatch(stripped, -1) {
		table := strings.TrimPrefix(strings.ToLower(match[1]), "striatumd.")
		tables[table] = true
	}
}
