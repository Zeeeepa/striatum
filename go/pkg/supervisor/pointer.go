// Package supervisor provides the Go-side supervisor lifecycle for RFC 0039
// Phase 2 Step 5. The Postgres-backed pointer row in
// striatumd.process_supervisor_pointers is the authoritative liveness record;
// the pidfile under .striatum/scratch/<supervisor_id>/pid is the local
// best-effort hint.
//
// The pointer row schema matches the Python supervisor (RFC 0043 §3) so the
// two implementations interoperate against the same Postgres substrate.
package supervisor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// PointerStore is the minimal Postgres surface this package needs. The
// concrete implementation lives in go/pkg/db; tests substitute an in-memory
// fake.
type PointerStore interface {
	UpsertSupervisorPointer(ctx context.Context, row PointerRow) error
	MarkSupervisorLost(ctx context.Context, supervisorID string, reason string) error
	GetSupervisorPointer(ctx context.Context, supervisorID string) (PointerRow, error)
}

// PointerRow mirrors striatumd.process_supervisor_pointers. The
// last_heartbeat_at field is what the daemon scans for stale-lease recovery.
type PointerRow struct {
	SupervisorID    string
	RepositoryID    string
	SessionID       string
	PID             int
	StartedAt       time.Time
	LastHeartbeatAt time.Time
	StdinPipePath   string
	State           string
	LostReason      string
}

// WritePidfile writes the pid to <scratch>/<supervisor_id>/pid atomically via
// rename. Mirrors the Python behavior in src/striatum/process/supervisor.py so
// the two implementations produce interchangeable on-disk hints.
func WritePidfile(scratchDir string, supervisorID string, pid int) (string, error) {
	dir := filepath.Join(scratchDir, supervisorID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("supervisor: mkdir scratch: %w", err)
	}
	pidPath := filepath.Join(dir, "pid")
	tmp := pidPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(strconv.Itoa(pid)+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("supervisor: write pid tmp: %w", err)
	}
	if err := os.Rename(tmp, pidPath); err != nil {
		return "", fmt.Errorf("supervisor: rename pidfile: %w", err)
	}
	return pidPath, nil
}

// ReadPidfile returns the pid recorded at <scratch>/<supervisor_id>/pid, or
// (0, os.ErrNotExist) if absent. The presence of a pidfile is not proof of
// liveness; pair with PointerRow.LastHeartbeatAt and an os.FindProcess +
// signal-0 probe (see liveness.go).
func ReadPidfile(scratchDir string, supervisorID string) (int, error) {
	pidPath := filepath.Join(scratchDir, supervisorID, "pid")
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, err
	}
	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return 0, fmt.Errorf("supervisor: parse pidfile %q: %w", pidPath, err)
	}
	return pid, nil
}

// UpsertPointer records the canonical liveness row in Postgres. Callers
// invoke this on supervisor.starting + supervisor.started events and on each
// heartbeat tick.
func UpsertPointer(ctx context.Context, store PointerStore, row PointerRow) error {
	if row.SupervisorID == "" {
		return fmt.Errorf("supervisor: empty supervisor_id")
	}
	if row.RepositoryID == "" {
		return fmt.Errorf("supervisor: empty repository_id")
	}
	if row.State == "" {
		row.State = "starting"
	}
	if row.LastHeartbeatAt.IsZero() {
		row.LastHeartbeatAt = time.Now().UTC()
	}
	return store.UpsertSupervisorPointer(ctx, row)
}
