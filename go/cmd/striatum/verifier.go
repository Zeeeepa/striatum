package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/halbritt/striatum/go/pkg/verifier"
)

// runVerifier implements `striatum verifier run`: the LANE-SIDE entrypoint of
// the RFC 0134 / D227 executable verification half. It runs INSIDE the
// disposable sandboxed verifier lane (a supervised subprocess off the daemon's
// gate path) — never in the daemon. It:
//
//  1. resolves the named check against the operator-curated, content-addressed
//     allowlist (a workflow NAMES a check; it never AUTHORS the executed bytes),
//  2. verifies the resolved binary's sha256 against the pinned hash,
//  3. runs the check TWICE under the strictest available sandbox envelope (the
//     two-signal independent re-execution agreement), and
//  4. mints a tamper-evident receipt.v1 (argv + binary sha + exit code + stdout
//     digest + cwd tree-sha + seal), writing it to --out for the lane to publish
//     as a `receipt` artifact.
//
// The daemon later READS that sealed receipt to allow a claim's VERIFIED status;
// it never re-runs the check. This command performs the only command execution
// in the whole feature, and it is firmly off the gate path.
//
// Flags:
//
//	--allowlist <path>  operator-curated, git-tracked allowlist JSON (required)
//	--check-id   <id>   the allowlist check to run (required)
//	--cwd        <dir>  worktree the check runs against (default: $PWD)
//	--scratch    <dir>  the single writable scratch dir inside the sandbox
//	--out        <path> where to write the minted receipt.v1 markdown (required)
//	--json              emit a machine-readable summary to stdout
func runVerifier(args []string, stdout io.Writer, stderr io.Writer, repoRootOverride string) int {
	if len(args) == 0 || args[0] != "run" {
		_, _ = fmt.Fprintln(stderr, "usage: striatum verifier run --allowlist <path> --check-id <id> --out <path> [--cwd <dir>] [--scratch <dir>] [--json]")
		return 2
	}
	var (
		allowlistPath string
		checkID       string
		cwd           string
		scratch       string
		outPath       string
		jsonOut       bool
	)
	rest := args[1:]
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--allowlist":
			i++
			if i < len(rest) {
				allowlistPath = rest[i]
			}
		case "--check-id":
			i++
			if i < len(rest) {
				checkID = rest[i]
			}
		case "--cwd":
			i++
			if i < len(rest) {
				cwd = rest[i]
			}
		case "--scratch":
			i++
			if i < len(rest) {
				scratch = rest[i]
			}
		case "--out":
			i++
			if i < len(rest) {
				outPath = rest[i]
			}
		case "--json":
			jsonOut = true
		case "-h", "--help":
			_, _ = fmt.Fprintln(stdout, "usage: striatum verifier run --allowlist <path> --check-id <id> --out <path> [--cwd <dir>] [--scratch <dir>] [--json]")
			return 0
		default:
			_, _ = fmt.Fprintf(stderr, "verifier run: unknown flag %q\n", rest[i])
			return 2
		}
	}
	if strings.TrimSpace(allowlistPath) == "" || strings.TrimSpace(checkID) == "" || strings.TrimSpace(outPath) == "" {
		_, _ = fmt.Fprintln(stderr, "verifier run: --allowlist, --check-id, and --out are required")
		return 2
	}
	if strings.TrimSpace(cwd) == "" {
		cwd = repoRootOverride
	}

	allowlist, err := verifier.LoadAllowlist(allowlistPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "verifier run: %v\n", err)
		return 1
	}
	result, err := verifier.ExecuteCheck(context.Background(), allowlist, verifier.RunRequest{
		CheckID:    checkID,
		Cwd:        cwd,
		ScratchDir: scratch,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "verifier run: %v\n", err)
		return 1
	}
	doc := result.Receipt.MarshalFrontMatter()
	if err := os.WriteFile(outPath, []byte(doc), 0o644); err != nil { //nolint:gosec // receipt is a non-secret durable artifact written into the lane worktree
		_, _ = fmt.Fprintf(stderr, "verifier run: write receipt %q: %v\n", outPath, err)
		return 1
	}

	if jsonOut {
		summary := map[string]any{
			"check_id":          result.Receipt.CheckID,
			"exit_code":         result.Receipt.ExitCode,
			"passed":            result.Passed,
			"classification":    result.Classification,
			"seal_digest":       result.Receipt.SealDigest,
			"cwd_tree_sha":      result.Receipt.CwdTreeSHA,
			"sandbox_mechanism": result.Receipt.Posture.Mechanism,
			"sandbox_strict":    result.Receipt.Posture.Strict,
			"agreement":         result.Receipt.AgreementSignal,
			"receipt_path":      outPath,
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(summary)
	} else {
		_, _ = fmt.Fprintf(stdout, "verifier run: check %q exit=%d passed=%t classification=%s mechanism=%s strict=%t agreement=%t\nreceipt: %s\n",
			result.Receipt.CheckID, result.Receipt.ExitCode, result.Passed, result.Classification,
			result.Receipt.Posture.Mechanism, result.Receipt.Posture.Strict, result.Receipt.AgreementSignal, outPath)
	}
	// The command's own exit code reflects the mechanical verdict so a lane can
	// branch on it, but the DURABLE truth is the sealed receipt, not this code.
	if !result.Passed {
		return 1
	}
	return 0
}
