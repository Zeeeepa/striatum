package verifier

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Receipt is the in-memory form of a receipt.v1 artifact (the tamper-evident
// transcript a verifier run mints). Its fields mirror the
// artifactcontracts "receipt" schema exactly, so MarshalFrontMatter produces a
// document the publisher accepts. SealDigest is the receipt's own content
// digest, computed over the canonical transcript so any tamper changes the seal.
type Receipt struct {
	CheckID      string
	Argv         []string
	BinarySHA256 string
	ExitCode     int
	StdoutSHA256 string
	CwdTreeSHA   string
	SealDigest   string
	CreatedAt    string
	// Posture and AgreementSignal are recorded in the receipt BODY (not the
	// schema-required front matter) so the gate reader can apply the two-signal
	// rule. They are part of what the seal digest covers.
	Posture         Posture
	AgreementSignal bool
}

// CheckResult is the per-check outcome the verifier lane derives mechanically.
// The verdict is DERIVED from the exit code, never authored. Classification maps
// to the claim-status lattice the gate reads (see EffectiveStatusFromReceipt).
type CheckResult struct {
	Receipt Receipt
	// Passed is the mechanical pass condition outcome (exit_zero).
	Passed bool
	// Classification is one of "verified_eligible" (strict sandbox, two-signal
	// agreement, exit-0), "asserted" (a lone exit-0 or a non-strict sandbox),
	// or "indeterminate" (timeout / envelope violation / network touch). It maps
	// to the lattice rung a receipt can EARN — never above ASSERTED unless both
	// signals hold under a strict envelope.
	Classification string
}

const (
	classVerifiedEligible = "verified_eligible"
	classAsserted         = "asserted"
	classIndeterminate    = "indeterminate"
)

// RunRequest is a single verifier check execution request (issued by the lane).
type RunRequest struct {
	// CheckID names an allowlist entry; the executed bytes come from the
	// allowlist, never from the request.
	CheckID string
	// Cwd is the worktree the check runs against (read-only in the sandbox).
	Cwd string
	// ScratchDir is the single writable directory inside the sandbox.
	ScratchDir string
	// Limits override the default sandbox caps (0 fields use defaults).
	Limits SandboxLimits
}

// ExecuteCheck runs a single allowlisted check TWICE under the strict sandbox
// envelope and mints a receipt. The two executions are the D227 "two signals":
// a sealed receipt PLUS an independent re-execution agreement. Both must reach
// the same exit code (and pass) under a STRICT sandbox for the check to be
// VERIFIED-eligible; a lone or disagreeing pass earns at most ASSERTED; a
// timeout / envelope violation / network touch is INDETERMINATE.
//
// THIS RUNS IN THE VERIFIER LANE, OFF THE DAEMON GATE PATH. The daemon never
// calls ExecuteCheck; it only reads the sealed receipt this produces.
func ExecuteCheck(ctx context.Context, allowlist *Allowlist, req RunRequest) (CheckResult, error) {
	entry, err := allowlist.Resolve(req.CheckID)
	if err != nil {
		return CheckResult{}, err
	}
	binarySHA, err := VerifyBinary(entry)
	if err != nil {
		return CheckResult{}, err
	}
	cwd := req.Cwd
	if strings.TrimSpace(cwd) == "" {
		cwd, _ = os.Getwd()
	}
	treeSHA, err := CwdTreeSHA(ctx, cwd)
	if err != nil {
		return CheckResult{}, fmt.Errorf("compute cwd tree-sha: %w", err)
	}
	wrapper, posture := ResolveSandbox(SandboxSpec{
		CwdReadOnly:     cwd,
		ScratchWritable: req.ScratchDir,
		Limits:          req.Limits,
	})

	first, err := runOnce(ctx, wrapper, entry.Argv, cwd, req.Limits)
	if err != nil {
		return CheckResult{}, err
	}
	second, err := runOnce(ctx, wrapper, entry.Argv, cwd, req.Limits)
	if err != nil {
		return CheckResult{}, err
	}

	// The recorded receipt binds to the FIRST execution's transcript; the second
	// is the independent agreement signal. Re-confirm the tree did not drift
	// between executions (a check that mutated its read-only-bound cwd is a
	// violation → INDETERMINATE).
	postTreeSHA, _ := CwdTreeSHA(ctx, cwd)
	treeStable := postTreeSHA == treeSHA

	agreement := first.exitCode == second.exitCode &&
		first.stdoutSHA == second.stdoutSHA &&
		!first.timedOut && !second.timedOut
	passed := entry.PassWhen == passWhenExitZero && first.exitCode == 0

	receipt := Receipt{
		CheckID:         entry.ID,
		Argv:            append([]string(nil), entry.Argv...),
		BinarySHA256:    binarySHA,
		ExitCode:        first.exitCode,
		StdoutSHA256:    first.stdoutSHA,
		CwdTreeSHA:      treeSHA,
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
		Posture:         posture,
		AgreementSignal: agreement,
	}
	receipt.SealDigest = receipt.computeSeal()

	classification := classifyResult(posture, first, agreement, treeStable, passed)
	return CheckResult{Receipt: receipt, Passed: passed, Classification: classification}, nil
}

// classifyResult maps an execution to the lattice rung its receipt can EARN.
// This is the heart of the two-signal rule: VERIFIED requires (a) a STRICT
// sandbox, (b) the independent re-execution AGREEMENT, (c) a stable read-only
// tree, and (d) a passing exit-0. Anything that touches the network, violates
// the envelope, or times out is INDETERMINATE — never VERIFIED. A lone exit-0
// (no strict envelope or no agreement) earns only ASSERTED.
func classifyResult(posture Posture, first runOutcome, agreement, treeStable, passed bool) string {
	if first.timedOut || first.envelopeViolation || !treeStable {
		return classIndeterminate
	}
	if posture.Strict && agreement && passed {
		return classVerifiedEligible
	}
	if passed {
		return classAsserted
	}
	// A failing check is a real, recorded result, not an error; the gate reads
	// the exit code. It earns no upgrade.
	return classAsserted
}

type runOutcome struct {
	exitCode          int
	stdoutSHA         string
	timedOut          bool
	envelopeViolation bool
}

// runOnce executes the wrapped check once with a wall-clock deadline, hashing
// stdout. A non-zero exit is a normal result (recorded), NOT a Go error; a Go
// error is reserved for failures to launch the sandbox itself.
func runOnce(ctx context.Context, wrapper, argv []string, cwd string, limits SandboxLimits) (runOutcome, error) {
	deadline := limits.withDefaults().WallClockSeconds
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(deadline)*time.Second)
	defer cancel()

	full := append(append([]string(nil), wrapper...), argv...)
	cmd := exec.CommandContext(execCtx, full[0], full[1:]...) //nolint:gosec // argv is the allowlisted, hash-pinned command wrapped by the resolved sandbox
	cmd.Dir = cwd
	// Minimal env: no inherited secrets. PATH only, so a network-resolving tool
	// is the only way out and the namespace already blocks it.
	cmd.Env = []string{"PATH=/usr/local/bin:/usr/bin:/bin", "HOME=" + cwd}

	var stdout hashingWriter
	cmd.Stdout = &stdout
	cmd.Stderr = &hashingWriter{} // stderr digest is not part of the receipt; discard

	runErr := cmd.Run()
	out := runOutcome{stdoutSHA: stdout.sum()}
	if execCtx.Err() == context.DeadlineExceeded {
		out.timedOut = true
		out.exitCode = -1
		return out, nil
	}
	if runErr == nil {
		out.exitCode = 0
		return out, nil
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		out.exitCode = exitErr.ExitCode()
		return out, nil
	}
	// A non-ExitError (e.g. the sandbox binary itself failed to start, or a
	// permission/namespace error) is an envelope violation — the check could not
	// run inside its envelope, so its result is INDETERMINATE, not a pass/fail.
	out.envelopeViolation = true
	out.exitCode = -1
	return out, nil
}

// CwdTreeSHA computes a content tree-sha over the worktree using git's plumbing
// against a TEMPORARY index, so it binds to the full working-tree contents (not
// just HEAD). It shells out to git from the LANE, never the daemon. Falls back
// to a deterministic recursive content hash when the dir is not a git tree.
func CwdTreeSHA(ctx context.Context, cwd string) (string, error) {
	if strings.TrimSpace(cwd) == "" {
		return "", fmt.Errorf("cwd is empty")
	}
	tmpIndex, err := os.CreateTemp("", "verifier-index-*")
	if err == nil {
		_ = tmpIndex.Close()
		defer func() { _ = os.Remove(tmpIndex.Name()) }()
		env := append(os.Environ(), "GIT_INDEX_FILE="+tmpIndex.Name())
		add := exec.CommandContext(ctx, "git", "-C", cwd, "add", "-A")
		add.Env = env
		if add.Run() == nil {
			wt := exec.CommandContext(ctx, "git", "-C", cwd, "write-tree")
			wt.Env = env
			if out, werr := wt.Output(); werr == nil {
				sha := strings.TrimSpace(string(out))
				if sha != "" {
					return sha, nil
				}
			}
		}
	}
	return recursiveContentSHA(cwd)
}

// recursiveContentSHA is the non-git fallback: a deterministic sha256 over the
// sorted relative paths + content of every regular file under root (skipping
// .git). Deterministic so two runs over an unchanged tree agree.
func recursiveContentSHA(root string) (string, error) {
	type fileHash struct {
		rel string
		sum string
	}
	var files []fileHash
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		data, readErr := os.ReadFile(path) //nolint:gosec // hashing the worktree the lane already has read access to
		if readErr != nil {
			return readErr
		}
		rel, _ := filepath.Rel(root, path)
		sum := sha256.Sum256(data)
		files = append(files, fileHash{rel: filepath.ToSlash(rel), sum: hex.EncodeToString(sum[:])})
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].rel < files[j].rel })
	h := sha256.New()
	for _, f := range files {
		h.Write([]byte(f.rel))
		h.Write([]byte{0})
		h.Write([]byte(f.sum))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// computeSeal hashes the canonical transcript (every field that makes the
// receipt tamper-evident, including posture and the agreement signal). Any edit
// to the recorded transcript changes the seal, so a hand-tampered receipt cannot
// keep its seal_digest — and a claim binding evidence_digest to the old seal
// auto-decays.
func (r Receipt) computeSeal() string {
	h := sha256.New()
	write := func(parts ...string) {
		for _, p := range parts {
			h.Write([]byte(p))
			h.Write([]byte{0})
		}
	}
	write("striatum.receipt.v1", r.CheckID)
	write(r.Argv...)
	write(r.BinarySHA256, strconv.Itoa(r.ExitCode), r.StdoutSHA256, r.CwdTreeSHA)
	write(string(r.Posture.Mechanism), strconv.FormatBool(r.Posture.Strict), strconv.FormatBool(r.AgreementSignal))
	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}

// hashingWriter is an io.Writer that streams its bytes into a sha256, so the
// receipt records a stdout digest without buffering the whole stream.
type hashingWriter struct{ h hash.Hash }

func (w *hashingWriter) Write(p []byte) (int, error) {
	if w.h == nil {
		w.h = sha256.New()
	}
	return w.h.Write(p)
}

func (w *hashingWriter) sum() string {
	if w.h == nil {
		w.h = sha256.New()
	}
	return hex.EncodeToString(w.h.Sum(nil))
}
