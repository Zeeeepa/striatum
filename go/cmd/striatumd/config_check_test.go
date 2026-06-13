package main

import (
	"strings"
	"testing"
)

const testDSN = "postgres://u@/db?host=/var/run/postgresql"

// disableBlobEnv clears every blob env var so daemonConfigProblems treats blob
// storage as opt-out (ErrConfigDisabled, not a problem) unless a test sets it.
func disableBlobEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"STRIATUM_BLOB_ENDPOINT", "STRIATUM_BLOB_ACCESS_KEY", "STRIATUM_BLOB_SECRET_KEY",
		"STRIATUM_BLOB_PATH_STYLE", "STRIATUM_BLOB_REGION", "STRIATUM_BLOB_BUCKET_PREFIX",
	} {
		t.Setenv(k, "")
	}
}

func TestDaemonConfigProblemsValidConfig(t *testing.T) {
	disableBlobEnv(t)
	if problems := daemonConfigProblems(testDSN, "", "v2"); len(problems) != 0 {
		t.Fatalf("valid config reported problems: %v", problems)
	}
	if code := runConfigCheck(testDSN, "", "v2"); code != 0 {
		t.Fatalf("runConfigCheck exit = %d, want 0", code)
	}
}

func TestDaemonConfigProblemsMalformedBlobBool(t *testing.T) {
	disableBlobEnv(t)
	t.Setenv("STRIATUM_BLOB_ENDPOINT", "http://localhost:9000")
	t.Setenv("STRIATUM_BLOB_ACCESS_KEY", "k")
	t.Setenv("STRIATUM_BLOB_SECRET_KEY", "s")
	// The exact value that crash-looped the daemon: a human note where a strict
	// boolean was required.
	t.Setenv("STRIATUM_BLOB_PATH_STYLE", "yes (use addressing_style=path)")

	problems := daemonConfigProblems(testDSN, "", "v2")
	if len(problems) != 1 {
		t.Fatalf("malformed blob bool: got %d problems, want 1: %v", len(problems), problems)
	}
	if !strings.Contains(problems[0].Error(), "STRIATUM_BLOB_PATH_STYLE") {
		t.Fatalf("problem did not name the offending field: %v", problems[0])
	}
	if code := runConfigCheck(testDSN, "", "v2"); code != exitConfigError {
		t.Fatalf("runConfigCheck exit = %d, want %d (EX_CONFIG)", code, exitConfigError)
	}
}

func TestDaemonConfigProblemsInvalidWriteBoundary(t *testing.T) {
	disableBlobEnv(t)
	problems := daemonConfigProblems(testDSN, "bogus_phase", "v2")
	if len(problems) != 1 {
		t.Fatalf("invalid write-boundary: got %d problems, want 1: %v", len(problems), problems)
	}
	if !strings.Contains(problems[0].Error(), "pg-write-boundary") {
		t.Fatalf("problem did not name the write-boundary flag: %v", problems[0])
	}
}

func TestDaemonConfigProblemsMissingURL(t *testing.T) {
	disableBlobEnv(t)
	t.Setenv("STRIATUM_DAEMON_DB_URL", "")
	// Point config resolution at a directory with no daemon.toml so the only URL
	// source is the (empty) flag/env — otherwise a real ~/.config/striatum/daemon.toml
	// on the dev box would satisfy it.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	problems := daemonConfigProblems("", "", "v2")
	if len(problems) != 1 {
		t.Fatalf("missing URL: got %d problems, want 1: %v", len(problems), problems)
	}
	if !strings.Contains(problems[0].Error(), "PostgreSQL URL") {
		t.Fatalf("problem did not name the missing URL: %v", problems[0])
	}
}

func TestDaemonConfigProblemsAggregatesMultiple(t *testing.T) {
	disableBlobEnv(t)
	t.Setenv("STRIATUM_BLOB_ENDPOINT", "http://localhost:9000")
	t.Setenv("STRIATUM_BLOB_ACCESS_KEY", "k")
	t.Setenv("STRIATUM_BLOB_SECRET_KEY", "s")
	t.Setenv("STRIATUM_BLOB_PATH_STYLE", "maybe")

	// Both a bad write-boundary and a bad blob value: the validator must report
	// both, so one fix does not just reveal the next.
	problems := daemonConfigProblems(testDSN, "bogus_phase", "v2")
	if len(problems) != 2 {
		t.Fatalf("aggregation: got %d problems, want 2: %v", len(problems), problems)
	}
}
