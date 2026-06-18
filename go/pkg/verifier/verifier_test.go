package verifier

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/artifactcontracts"
)

func writeAllowlist(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "allowlist.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write allowlist: %v", err)
	}
	return path
}

// TestAllowlistRejectsMalformed verifies the allowlist fails closed on a bad
// schema marker, an empty checks list, a missing argv, a missing binary hash,
// a duplicate id, and an unsupported pass_when — an operator cannot author a
// malformed or silently-unenforced check.
func TestAllowlistRejectsMalformed(t *testing.T) {
	cases := map[string]string{
		"bad schema":     `{"schema_version":"nope","checks":[{"id":"a","argv":["/bin/true"],"binary_sha256":"x"}]}`,
		"empty checks":   `{"schema_version":"striatum.verifier_allowlist.v1","checks":[]}`,
		"empty argv":     `{"schema_version":"striatum.verifier_allowlist.v1","checks":[{"id":"a","argv":[],"binary_sha256":"x"}]}`,
		"no binary sha":  `{"schema_version":"striatum.verifier_allowlist.v1","checks":[{"id":"a","argv":["/bin/true"]}]}`,
		"unsupported pw": `{"schema_version":"striatum.verifier_allowlist.v1","checks":[{"id":"a","argv":["/bin/true"],"binary_sha256":"x","pass_when":"stdout_contains"}]}`,
		"dup id":         `{"schema_version":"striatum.verifier_allowlist.v1","checks":[{"id":"a","argv":["/bin/true"],"binary_sha256":"x"},{"id":"a","argv":["/bin/true"],"binary_sha256":"y"}]}`,
	}
	for name, body := range cases {
		if _, err := ParseAllowlist([]byte(body)); err == nil {
			t.Fatalf("%s: malformed allowlist must be rejected", name)
		}
	}
}

// TestAllowlistResolveRefusesUnknownCheck is the "NAME but never AUTHOR" rule:
// a check not in the curated allowlist is refused before any binary is touched.
func TestAllowlistResolveRefusesUnknownCheck(t *testing.T) {
	al, err := ParseAllowlist([]byte(`{"schema_version":"striatum.verifier_allowlist.v1","checks":[{"id":"known","argv":["/bin/true"],"binary_sha256":"abc"}]}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := al.Resolve("unknown"); err == nil || !strings.Contains(err.Error(), "not in the operator-curated") {
		t.Fatalf("unknown check must be refused: %v", err)
	}
	if _, err := al.Resolve("known"); err != nil {
		t.Fatalf("known check must resolve: %v", err)
	}
}

// TestVerifyBinaryRefusesDriftedHash verifies content addressing: a binary whose
// on-disk sha256 differs from the pinned hash is refused (a swapped/tampered
// binary).
func TestVerifyBinaryRefusesDriftedHash(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "tool")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatalf("write tool: %v", err)
	}
	actual := fileSHA(t, bin)

	good := AllowlistEntry{ID: "t", Argv: []string{bin}, BinarySHA256: actual}
	if got, err := VerifyBinary(good); err != nil || got != actual {
		t.Fatalf("matching hash must pass: got=%q err=%v", got, err)
	}
	bad := AllowlistEntry{ID: "t", Argv: []string{bin}, BinarySHA256: "deadbeef"}
	if _, err := VerifyBinary(bad); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("drifted hash must be refused: %v", err)
	}
}

// TestResolveSandboxSelectsStrongestAvailable verifies mechanism selection is
// strongest-first and that the posture honestly reports which guarantees hold.
func TestResolveSandboxSelectsStrongestAvailable(t *testing.T) {
	orig := lookPath
	defer func() { lookPath = orig }()

	// All present → bubblewrap, strict.
	lookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	_, posture := ResolveSandbox(SandboxSpec{CwdReadOnly: "/x", ScratchWritable: "/x/scratch"})
	if posture.Mechanism != MechanismBubblewrap || !posture.Strict {
		t.Fatalf("with bwrap present, expected strict bubblewrap, got %+v", posture)
	}
	if !posture.NoNetwork || !posture.NoNewPrivs || !posture.ReadOnlyRootFS || !posture.CgroupCaps {
		t.Fatalf("strict bubblewrap must assert all guarantees, got %+v", posture)
	}

	// Only systemd-run → non-strict (no read-only rootfs).
	lookPath = func(name string) (string, error) {
		if name == "systemd-run" {
			return "/usr/bin/systemd-run", nil
		}
		return "", os.ErrNotExist
	}
	_, posture = ResolveSandbox(SandboxSpec{})
	if posture.Mechanism != MechanismSystemdRun || posture.Strict {
		t.Fatalf("systemd-run-only must be non-strict, got %+v", posture)
	}

	// Only unshare → non-strict.
	lookPath = func(name string) (string, error) {
		if name == "unshare" {
			return "/usr/bin/unshare", nil
		}
		return "", os.ErrNotExist
	}
	_, posture = ResolveSandbox(SandboxSpec{})
	if posture.Mechanism != MechanismUnshareUlimit || posture.Strict {
		t.Fatalf("unshare-only must be non-strict, got %+v", posture)
	}

	// Nothing → MechanismNone, never strict (cannot earn VERIFIED).
	lookPath = func(string) (string, error) { return "", os.ErrNotExist }
	_, posture = ResolveSandbox(SandboxSpec{})
	if posture.Mechanism != MechanismNone || posture.Strict {
		t.Fatalf("no primitive must be MechanismNone, non-strict, got %+v", posture)
	}
}

// TestEffectiveStatusFromReceiptTwoSignal is the heart of the two-signal rule:
// VERIFIED requires the sealed receipt bound to THESE inputs AND a strict sandbox
// AND the independent re-execution agreement AND exit-0. Every weaker case is
// ASSERTED — a lone exit-0 never reaches the top rung.
func TestEffectiveStatusFromReceiptTwoSignal(t *testing.T) {
	const digest = "treesha_abc"
	base := ReceiptSignals{SealDigest: digest, ExitCode: 0, Strict: true, Agreement: true}

	// Both signals, strict, bound, exit-0 → VERIFIED.
	if got := EffectiveStatusFromReceipt(base, digest, digest); got != artifactcontracts.ClaimStatusVerified {
		t.Fatalf("two-signal strict bound exit-0 must be VERIFIED, got %q", got)
	}
	// No agreement → ASSERTED (lone signal).
	noAgree := base
	noAgree.Agreement = false
	if got := EffectiveStatusFromReceipt(noAgree, digest, digest); got != artifactcontracts.ClaimStatusAsserted {
		t.Fatalf("a lone exit-0 (no agreement) must be ASSERTED, got %q", got)
	}
	// Non-strict sandbox → ASSERTED.
	notStrict := base
	notStrict.Strict = false
	if got := EffectiveStatusFromReceipt(notStrict, digest, digest); got != artifactcontracts.ClaimStatusAsserted {
		t.Fatalf("a non-strict sandbox must cap at ASSERTED, got %q", got)
	}
	// Non-zero exit → ASSERTED.
	failing := base
	failing.ExitCode = 1
	if got := EffectiveStatusFromReceipt(failing, digest, digest); got != artifactcontracts.ClaimStatusAsserted {
		t.Fatalf("a non-zero exit must be ASSERTED, got %q", got)
	}
	// Seal does not bind these inputs (evidence != bound) → ASSERTED (decayed).
	if got := EffectiveStatusFromReceipt(base, "treesha_new", digest); got != artifactcontracts.ClaimStatusAsserted {
		t.Fatalf("a receipt binding stale inputs must decay to ASSERTED, got %q", got)
	}
	// Receipt seal does not match the claim's evidence_digest → ASSERTED.
	mismatched := base
	mismatched.SealDigest = "other"
	if got := EffectiveStatusFromReceipt(mismatched, digest, digest); got != artifactcontracts.ClaimStatusAsserted {
		t.Fatalf("a receipt whose seal != evidence must be ASSERTED, got %q", got)
	}
	// Empty digests → ASSERTED.
	if got := EffectiveStatusFromReceipt(base, "", ""); got != artifactcontracts.ClaimStatusAsserted {
		t.Fatalf("an unbound claim must be ASSERTED, got %q", got)
	}
}

// TestReceiptRoundTripIsSchemaValidAndReadable verifies a minted receipt both
// validates against the receipt.v1 contract AND parses back into the gate-read
// signal projection with the strict/agreement markers intact (the seal-covered
// body markers the gate reads).
func TestReceiptRoundTripIsSchemaValidAndReadable(t *testing.T) {
	r := Receipt{
		CheckID:         "go-test",
		Argv:            []string{"/bin/true"},
		BinarySHA256:    "abc",
		ExitCode:        0,
		StdoutSHA256:    "def",
		CwdTreeSHA:      "treesha_abc",
		CreatedAt:       "2026-06-18T00:00:00Z",
		Posture:         Posture{Mechanism: MechanismBubblewrap, Strict: true},
		AgreementSignal: true,
	}
	r.SealDigest = r.computeSeal()
	doc := r.MarshalFrontMatter()

	if err := artifactcontracts.ValidateFrontMatter("receipt", "RECEIPT.md", []byte(doc)); err != nil {
		t.Fatalf("minted receipt must validate against the receipt.v1 contract: %v", err)
	}
	sig, err := ReceiptSignalsFromDocument(doc)
	if err != nil {
		t.Fatalf("gate reader must parse the receipt: %v", err)
	}
	if sig.SealDigest != r.SealDigest || !sig.Strict || !sig.Agreement || sig.ExitCode != 0 {
		t.Fatalf("gate-read signals must match the minted receipt: %+v", sig)
	}
	// A non-strict / no-agreement receipt reads back with those signals false.
	r2 := r
	r2.Posture.Strict = false
	r2.AgreementSignal = false
	r2.SealDigest = r2.computeSeal()
	sig2, err := ReceiptSignalsFromDocument(r2.MarshalFrontMatter())
	if err != nil {
		t.Fatalf("parse second receipt: %v", err)
	}
	if sig2.Strict || sig2.Agreement {
		t.Fatalf("a non-strict no-agreement receipt must read those signals false: %+v", sig2)
	}
}

// TestExecuteCheckMintsReceiptUnderNonePosture is the CI-portable core of the
// end-to-end lane path: with the sandbox resolver forced to MechanismNone (no
// host primitive), a passing allowlisted check still runs deterministically and
// mints a sealed receipt whose seal equals its content digest and whose tree-sha
// binds the worktree. The non-strict posture is the load-bearing security
// assertion: a 'none' posture caps the receipt at ASSERTED — it can NEVER reach
// verified_eligible (only a strict envelope earns that). This path has no
// dependency on bwrap / a user systemd manager / unprivileged unshare, so it is
// deterministic in CI (degraded) AND on a bwrap host.
func TestExecuteCheckMintsReceiptUnderNonePosture(t *testing.T) {
	trueBin := firstExisting("/bin/true", "/usr/bin/true")
	if trueBin == "" {
		t.Skip("no /bin/true on host")
	}
	// Force the resolver to find no sandbox primitive, so ExecuteCheck runs the
	// check unwrapped under MechanismNone — a deterministic, non-sandboxed exec
	// path that is identical in every environment.
	orig := lookPath
	defer func() { lookPath = orig }()
	lookPath = func(string) (string, error) { return "", os.ErrNotExist }

	res := mintTrueReceipt(t, trueBin)

	if res.Receipt.Posture.Mechanism != MechanismNone || res.Receipt.Posture.Strict {
		t.Fatalf("forced-none posture must be MechanismNone and non-strict, got %+v", res.Receipt.Posture)
	}
	if !res.Passed || res.Receipt.ExitCode != 0 {
		t.Fatalf("a /bin/true check must pass even under the none posture: %+v", res)
	}
	// The security property: a non-strict (here, sandbox-less) run can never earn
	// VERIFIED — a lone exit-0 caps at ASSERTED no matter how clean the pass is.
	if res.Classification != classAsserted {
		t.Fatalf("a non-strict passing check must be classified asserted, got %q", res.Classification)
	}
}

// TestExecuteCheckHostSandboxPosture exercises the REAL resolver on whatever
// primitive this host actually has. It always asserts the receipt mints and is
// tamper-evident, then asserts only the invariant the resolved posture earns —
// never strict behavior unconditionally. On a strict-sandbox host (e.g. bwrap)
// it asserts the verified-eligible two-signal path; in a degraded environment
// (CI: no bwrap, no usable user systemd manager, restricted unshare) it asserts
// the fail-safe invariant that a non-strict posture cannot reach
// verified_eligible. If the resolved primitive is present on PATH but cannot
// actually launch the check here (a non-strict posture that did not produce a
// clean exit-0 — exactly the GitHub-runner systemd-run-user case), the strict
// pass assertions are skipped rather than asserted against an envelope that
// could not run.
func TestExecuteCheckHostSandboxPosture(t *testing.T) {
	trueBin := firstExisting("/bin/true", "/usr/bin/true")
	if trueBin == "" {
		t.Skip("no /bin/true on host")
	}
	res := mintTrueReceipt(t, trueBin)

	// Receipt minting is environment-independent: it must hold under every posture.
	if res.Receipt.SealDigest == "" || res.Receipt.CwdTreeSHA == "" {
		t.Fatalf("receipt must carry a seal and a tree-sha: %+v", res.Receipt)
	}
	if res.Receipt.SealDigest != res.Receipt.computeSeal() {
		t.Fatal("receipt seal must equal its recomputed content digest (tamper-evident)")
	}

	if res.Receipt.Posture.Strict {
		// A strict envelope MUST be able to run the check; under it a passing,
		// agreeing check is the two-signal VERIFIED-eligible mint.
		if !res.Passed || res.Receipt.ExitCode != 0 {
			t.Fatalf("a /bin/true check must pass under a strict sandbox: %+v", res)
		}
		if !res.Receipt.AgreementSignal || res.Classification != classVerifiedEligible {
			t.Fatalf("strict + passing + agreeing must be verified_eligible: %+v", res)
		}
		return
	}

	// Non-strict posture: the security floor is that it can never earn VERIFIED.
	if res.Classification == classVerifiedEligible {
		t.Fatalf("a non-strict posture must not be verified_eligible: %+v", res)
	}
	if res.Passed && res.Receipt.ExitCode == 0 {
		// The non-strict envelope actually ran the check cleanly → it must classify
		// as asserted (a lone exit-0, no strict envelope).
		if res.Classification != classAsserted {
			t.Fatalf("a non-strict passing check must be asserted, got %q", res.Classification)
		}
		return
	}
	// The resolved primitive is on PATH but could not launch the check in this
	// environment (e.g. `systemd-run --user --scope` with no usable user manager,
	// or unprivileged-userns-disabled `unshare`). The check did not pass, but the
	// receipt still minted honestly under a non-strict posture; there is no strict
	// behavior to assert here. This is the degraded-CI path.
	t.Logf("non-strict primitive %q could not run the check cleanly here (exit=%d, class=%q); "+
		"receipt minted non-strict, strict-path assertions skipped",
		res.Receipt.Posture.Mechanism, res.Receipt.ExitCode, res.Classification)
}

// mintTrueReceipt allowlists the host /bin/true under its real sha and runs
// ExecuteCheck over a seeded one-file worktree, returning the result.
func mintTrueReceipt(t *testing.T, trueBin string) CheckResult {
	t.Helper()
	sha := fileSHA(t, trueBin)
	al, err := ParseAllowlist([]byte(`{"schema_version":"striatum.verifier_allowlist.v1","checks":[{"id":"t","argv":["` + trueBin + `"],"binary_sha256":"` + sha + `"}]}`))
	if err != nil {
		t.Fatalf("parse allowlist: %v", err)
	}
	work := t.TempDir()
	if err := os.WriteFile(filepath.Join(work, "f.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("seed work: %v", err)
	}
	res, err := ExecuteCheck(context.Background(), al, RunRequest{CheckID: "t", Cwd: work, ScratchDir: t.TempDir()})
	if err != nil {
		t.Fatalf("ExecuteCheck: %v", err)
	}
	return res
}

func fileSHA(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // test reads a known path
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func firstExisting(paths ...string) string {
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
