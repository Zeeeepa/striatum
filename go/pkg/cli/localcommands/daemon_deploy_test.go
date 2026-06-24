package localcommands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
)

// TestRunDaemonDeployDispatch is F3 (verb-dispatch): `daemon deploy` routes to the
// deployer verb, an unknown flag is a usage error (exit 2), and the top-level
// daemon usage advertises `deploy`.
func TestRunDaemonDeployDispatch(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := RunDaemon([]string{"deploy", "--bogus-flag"}, &stdout, &stderr, "test"); code != 2 {
		t.Fatalf("deploy --bogus-flag exit = %d, want 2 (usage); stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "unknown daemon deploy flag") {
		t.Fatalf("unknown-flag stderr did not name the deploy verb: %q", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := RunDaemon(nil, &stdout, &stderr, "test"); code != 2 {
		t.Fatalf("daemon (no args) exit = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "deploy") {
		t.Fatalf("daemon usage does not advertise the deploy verb: %q", stderr.String())
	}
}

// TestRunDaemonDeployM3PreflightRefusesWithoutDecoupled is the M3 (e) verb arm:
// THIS binary embeds the DDL-revoke (owner bundle 0021), so `daemon deploy` with
// STRIATUM_DEPLOY_DECOUPLED unset must refuse BEFORE connecting, naming the flag —
// the activation preflight that keeps a revoke-embedding binary off the legacy
// mutate path.
func TestRunDaemonDeployM3PreflightRefusesWithoutDecoupled(t *testing.T) {
	embedded, err := db.RevokeBundleEmbedded()
	if err != nil {
		t.Fatalf("RevokeBundleEmbedded: %v", err)
	}
	if !embedded {
		t.Skip("this binary does not embed the DDL-revoke (pre-step-7 build); M3 verb preflight is inert")
	}
	t.Setenv(db.EnvDeployDecoupled, "")

	var stdout, stderr bytes.Buffer
	// A syntactically valid DSN passes the no-DSN guard so we reach the preflight;
	// the preflight refuses BEFORE any connection is attempted.
	code := RunDaemon([]string{"deploy", "--owner-url", "postgres://u@127.0.0.1:5432/db?sslmode=disable"}, &stdout, &stderr, "test")
	if code != 1 {
		t.Fatalf("revoke-embedding deploy without %s exit = %d, want 1; stderr=%q", db.EnvDeployDecoupled, code, stderr.String())
	}
	if !strings.Contains(stderr.String(), db.EnvDeployDecoupled) {
		t.Fatalf("M3 preflight refusal must name %s; got %q", db.EnvDeployDecoupled, stderr.String())
	}
}
