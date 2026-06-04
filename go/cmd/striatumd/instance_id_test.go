package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStableInstanceIDPersistsAcrossBoots is the GH #168 gate: the instance id
// is stable across restarts so the owner-owned daemon_auth_registry UPSERTs a
// single row per daemon installation instead of growing one row per process
// (which also tripped a false rotator_collision on a restart within the 5-minute
// probe window, RFC 0110 §9.4).
func TestStableInstanceIDPersistsAcrossBoots(t *testing.T) {
	dir := t.TempDir()

	first, err := stableInstanceID(dir)
	if err != nil {
		t.Fatalf("first boot: %v", err)
	}
	if !strings.HasPrefix(first, "inst-") {
		t.Fatalf("instance id %q lacks the inst- prefix", first)
	}

	// A second "boot" reading the same runtime dir must return the same id.
	second, err := stableInstanceID(dir)
	if err != nil {
		t.Fatalf("second boot: %v", err)
	}
	if second != first {
		t.Fatalf("instance id changed across boots: first=%q second=%q (would grow the registry)", first, second)
	}

	// The id is persisted in the runtime dir, 0600.
	path := filepath.Join(dir, instanceIDFileName)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("instance id not persisted: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("instance id file mode = %o, want 600", perm)
	}
}

// TestStableInstanceIDReplacesEmptyFile guards the empty/whitespace-file case:
// a truncated or blank instance-id file is replaced rather than returned as an
// empty registry key.
func TestStableInstanceIDReplacesEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, instanceIDFileName)
	if err := os.WriteFile(path, []byte("  \n"), 0o600); err != nil {
		t.Fatalf("seed empty file: %v", err)
	}
	id, err := stableInstanceID(dir)
	if err != nil {
		t.Fatalf("stableInstanceID: %v", err)
	}
	if strings.TrimSpace(id) == "" || !strings.HasPrefix(id, "inst-") {
		t.Fatalf("empty file not replaced with a valid id: %q", id)
	}
}

// TestStableInstanceIDDistinctRuntimeDirs confirms two distinct installations
// (different runtime dirs) get distinct ids — per-instance roles for a shared
// PostgreSQL stay distinguishable.
func TestStableInstanceIDDistinctRuntimeDirs(t *testing.T) {
	a, err := stableInstanceID(t.TempDir())
	if err != nil {
		t.Fatalf("dir a: %v", err)
	}
	b, err := stableInstanceID(t.TempDir())
	if err != nil {
		t.Fatalf("dir b: %v", err)
	}
	if a == b {
		t.Fatalf("distinct runtime dirs produced the same instance id %q", a)
	}
}
