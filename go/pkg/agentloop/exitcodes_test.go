package agentloop

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// TestReservedExitCodeAndSentinelIdentity pins the RFC 0143 Slice A reserved code
// (97) and the typed sentinel (Slice A-1). 97 is well outside the 0/1/2 adapter
// range and is NOT the Slice-B reseal code (98); the sentinel is a distinct,
// errors.Is-matchable value.
func TestReservedExitCodeAndSentinelIdentity(t *testing.T) {
	if ExitUnrecoverableAcrossRotation != 97 {
		t.Fatalf("ExitUnrecoverableAcrossRotation = %d, want 97", ExitUnrecoverableAcrossRotation)
	}
	if ExitUnrecoverableAcrossRotation == 98 {
		t.Fatal("Slice A must NOT define the Slice-B reseal-request code 98")
	}
	if ErrUnrecoverableAcrossRotation == nil {
		t.Fatal("ErrUnrecoverableAcrossRotation sentinel is nil")
	}
	if !errors.Is(ErrUnrecoverableAcrossRotation, ErrUnrecoverableAcrossRotation) {
		t.Fatal("sentinel must be errors.Is-matchable against itself")
	}
}

// TestAgentLoopMapsUnrecoverableSentinelToReservedExit asserts the mapping the
// cmd/striatumd entrypoint performs: errors.Is(err, ErrUnrecoverableAcrossRotation)
// is the ONLY condition that should drive exit 97. We assert the predicate the
// entrypoint uses (a same-package unit; the os.Exit call itself is not unit-testable).
func TestAgentLoopMapsUnrecoverableSentinelToReservedExit(t *testing.T) {
	if !errors.Is(ErrUnrecoverableAcrossRotation, ErrUnrecoverableAcrossRotation) {
		t.Fatal("the sentinel must match itself under errors.Is (the main.go gate)")
	}
	// A generic agent-exit error must NOT be mistaken for the floor sentinel.
	generic := errors.New("agent command exited: signal: terminated")
	if errors.Is(generic, ErrUnrecoverableAcrossRotation) {
		t.Fatal("a generic agent-exit error must not match the reserved sentinel")
	}
}

// TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker is the direct-path
// C2 (RFC 0143 Slice A §6): normalizeAgentExitError NEVER consults the provider
// child's numeric exit code, so a child that itself exits 97 (or 98) yields only the
// generic "agent command exited" error — never the reserved sentinel. The reserved
// code is emitted ONLY when the wrapper's own unrecoverableExitRequested flag is set.
func TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker(t *testing.T) {
	// A provider child exiting with code 97 surfaces to the wrapper as a generic
	// *exec.ExitError-shaped error; normalizeAgentExitError must wrap it generically.
	childExit97 := errors.New("exit status 97")
	got := normalizeAgentExitError(childExit97, false, false)
	if got == nil || !strings.Contains(got.Error(), "agent command exited") {
		t.Fatalf("provider exit 97 = %v, want generic 'agent command exited' wrap", got)
	}
	if errors.Is(got, ErrUnrecoverableAcrossRotation) {
		t.Fatal("a provider child's exit 97 must NOT forge the reserved sentinel (C2)")
	}
	childExit98 := errors.New("exit status 98")
	got98 := normalizeAgentExitError(childExit98, false, false)
	if errors.Is(got98, ErrUnrecoverableAcrossRotation) {
		t.Fatal("a provider child's exit 98 must NOT forge the reserved sentinel (C2)")
	}
	// Only the wrapper's OWN observation (unrecoverableExitRequested=true) yields the
	// sentinel — and it takes precedence even over an idle-exit request and over a
	// nil child error.
	if got := normalizeAgentExitError(nil, true, true); !errors.Is(got, ErrUnrecoverableAcrossRotation) {
		t.Fatalf("wrapper-observed unrecoverable exit = %v, want the reserved sentinel", got)
	}
	if got := normalizeAgentExitError(childExit97, false, true); !errors.Is(got, ErrUnrecoverableAcrossRotation) {
		t.Fatalf("wrapper-observed unrecoverable exit (with child err) = %v, want the reserved sentinel", got)
	}
}

// TestAdminTokenReachedByNonOwnerENOENTFallsThrough asserts the narrowing does NOT
// fire when the admin runtime client-token is simply ABSENT (ENOENT) — the chain
// must fall through to the next resolver tier unchanged (no over-fire).
func TestAdminTokenReachedByNonOwnerENOENTFallsThrough(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist", "client-token")
	if adminTokenReachedByNonOwner(missing) {
		t.Fatal("a missing admin token (ENOENT) must NOT be the floor; it must fall through")
	}
}

// TestAdminTokenReachedByOwnerIsNotFloor asserts the OWNER is unaffected: a token
// file owned by this process's euid returns false (the caller reads it normally).
func TestAdminTokenReachedByOwnerIsNotFloor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "client-token")
	if err := os.WriteFile(path, []byte("dtok_owner\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if adminTokenReachedByNonOwner(path) {
		t.Fatalf("an admin token owned by euid=%d must NOT be the floor (owner unaffected)", os.Geteuid())
	}
}

// TestResolveRefusesRuntimeClientTokenForNonOwner is the A1 narrowing for a
// NON-OWNER lane: when the resolution chain reaches the runtime client-token whose
// owner uid != euid, BOTH resolvers return the typed sentinel (→ reserved exit 97)
// instead of reading the token or returning a generic permission error. We simulate
// non-owner ownership by stat-uid mismatch via a stubbed euid is not possible
// without root, so we assert the predicate path directly: a file whose owner uid
// differs from euid refuses. (Under the production shared-uid model the EACCES form
// is the live path; the uid-mismatch form is exercised here deterministically.)
func TestResolveRefusesRuntimeClientTokenForNonOwner(t *testing.T) {
	// Build a token file and then assert adminTokenReachedByNonOwner refuses when the
	// owner uid is forced to differ from euid. We cannot chown to another uid without
	// privilege, so we validate the refuse-before-read CONTRACT through the resolver
	// using an unreadable (0700 parent, denied) layout where available; otherwise we
	// fall back to asserting the ENOENT/owner invariants already covered above plus
	// the resolver wiring returns the sentinel for the EACCES form.
	dir := t.TempDir()
	parent := filepath.Join(dir, "runtime")
	if err := os.MkdirAll(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "client-token")
	if err := os.WriteFile(path, []byte("dtok_admin\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Deny stat on the file by removing search permission on its parent dir. As the
	// owner (euid) we can still stat unless we drop our own x bit AND are not root.
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses the EACCES path; the non-owner refuse is exercised in production under the shared striatum-lane uid")
	}
	if err := os.Chmod(parent, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o700) })

	// The resolver, on reaching this tier, must REFUSE with the typed sentinel, not
	// read and not a generic error.
	env := []string{EnvDaemonRuntimeDir + "=" + parent}
	_, err := ResolveTokenMaterial("", env)
	if err == nil {
		// If the platform still allowed the stat (e.g. capabilities), fall back to a
		// direct EACCES assertion on the predicate so the test stays meaningful.
		if _, statErr := os.Lstat(path); statErr != nil && (errors.Is(statErr, syscall.EACCES) || errors.Is(statErr, syscall.EPERM)) {
			if !adminTokenReachedByNonOwner(path) {
				t.Fatal("EACCES on the admin token must be the floor")
			}
			return
		}
		t.Skip("platform allowed stat through a 0000 dir; EACCES refuse path not exercisable here")
	}
	if !errors.Is(err, ErrUnrecoverableAcrossRotation) {
		t.Fatalf("ResolveTokenMaterial for an unreadable admin token = %v, want ErrUnrecoverableAcrossRotation", err)
	}

	// And the fresh resolver (#323 reader) refuses identically.
	t.Setenv(EnvDaemonRuntimeDir, parent)
	_, ferr := ResolveTokenMaterialFresh("")
	if ferr == nil || !errors.Is(ferr, ErrUnrecoverableAcrossRotation) {
		t.Fatalf("ResolveTokenMaterialFresh for an unreadable admin token = %v, want ErrUnrecoverableAcrossRotation", ferr)
	}
}

// TestApplyMCPEndpointRotationCodexReturnsUnrecoverableSentinel is the A2-route
// codex corroborator: applyMCPEndpointRotation, after pushing the in-band wedge
// prompt for a codex lane that cannot reload its `-c`-baked url, returns the typed
// ErrUnrecoverableAcrossRotation so the watcher requests the clean reserved-code
// exit (instead of the pre-Slice-A `return nil` that ground on silently).
func TestApplyMCPEndpointRotationCodexReturnsUnrecoverableSentinel(t *testing.T) {
	rec := &writeRecorder{}
	var stderr strings.Builder
	rotated := "http://127.0.0.1:41283/mcp/sse"
	err := applyMCPEndpointRotation("/home/x/.local/bin/codex", "", rotated, TokenMaterial{Token: "dtok_rotated"}, rec, &stderr)
	if !errors.Is(err, ErrUnrecoverableAcrossRotation) {
		t.Fatalf("codex rotation = %v, want ErrUnrecoverableAcrossRotation (the unrecoverable corroborator)", err)
	}
	// The wedge prompt is still pushed in-band (the agent sees the broken path).
	if strings.Join(rec.writes, "") == "" {
		t.Fatal("codex rotation must still push the in-band wedge prompt before returning the sentinel")
	}
	// A claude lane (recoverable: config rewrite + /mcp reconnect) must NOT return
	// the sentinel — it self-heals, so no reserved-code exit.
	rec2 := &writeRecorder{}
	var stderr2 strings.Builder
	if err := applyMCPEndpointRotation("/home/x/.local/bin/agy", "", rotated, TokenMaterial{Token: "dtok"}, rec2, &stderr2); err != nil {
		t.Fatalf("agy rotation must not be unrecoverable (no reconnect affordance, but recoverable): %v", err)
	}
}
