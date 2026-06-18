// Package verifier implements the executable half of RFC 0134 / D227: the
// disposable, sandboxed verifier LANE that mints receipts, the operator-curated
// content-addressed check ALLOWLIST it is bound to, and the pure-read EVALUATION
// the run-completion gate uses to allow a claim's VERIFIED status.
//
// The cardinal rule of D227 lives here as code: the daemon VALIDATES, never
// EXECUTES. Every function in this package that runs a process is invoked from a
// supervised verifier LANE (a subprocess off the daemon's gate path), never from
// the daemon's own completion path. The daemon-side reader (EvaluateClaim,
// EffectiveStatusFromReceipt) only reads sealed receipts and compares digests —
// it shells out to nothing. Placing any Execute* call on the daemon's gate path
// would inherit the daemon's PG socket, runtime token, and cross-repo reach,
// which is precisely the RCE/exfil surface D227 rejects.
package verifier

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// AllowlistEntry is one operator-curated, git-tracked check the verifier lane is
// permitted to run. A workflow NAMES a check by its ID; it never AUTHORS the
// executed bytes. The entry pins the exact argv and the sha256 of the resolved
// binary (argv[0]), so a lane that names "go-test" cannot smuggle a different
// command or a swapped binary past the allowlist.
type AllowlistEntry struct {
	// ID is the stable check identifier a workflow's checks[] row references.
	ID string `json:"id"`
	// Argv is the exact, fully-resolved command to execute. argv[0] is the
	// binary whose on-disk sha256 must equal BinarySHA256 at resolution time.
	Argv []string `json:"argv"`
	// BinarySHA256 pins the sha256 of the resolved binary (argv[0]). The verifier
	// refuses to run a check whose binary's actual hash differs — content
	// addressing: the operator curated THESE bytes, not whatever is on PATH today.
	BinarySHA256 string `json:"binary_sha256"`
	// PassWhen is the mechanical pass condition. Only exit_zero is honored in this
	// slice (the verdict is derived, never authored); other conditions are
	// reserved and currently rejected at load so an operator cannot author a
	// silently-unenforced condition.
	PassWhen string `json:"pass_when"`
	// Description is operator-facing prose; it is informational only.
	Description string `json:"description,omitempty"`
}

// passWhenExitZero is the only mechanical pass condition honored in this slice.
const passWhenExitZero = "exit_zero"

// Allowlist is the operator-curated set of permitted checks, keyed by ID. It is
// loaded from a git-tracked JSON file; the verifier resolves a named check
// against it and refuses any check that is not present or whose binary content
// has drifted from the pinned hash.
type Allowlist struct {
	entries map[string]AllowlistEntry
}

// AllowlistFile is the on-disk JSON shape: a schema marker plus the checks list.
// It is a tracked operator artifact (NOT under .striatum/, which is scratch), so
// the curated set is durable provenance the run can be audited against.
type AllowlistFile struct {
	SchemaVersion string           `json:"schema_version"`
	Checks        []AllowlistEntry `json:"checks"`
}

// AllowlistSchemaVersion is the required schema_version marker on the file.
const AllowlistSchemaVersion = "striatum.verifier_allowlist.v1"

// LoadAllowlist reads and validates an operator-curated allowlist JSON file.
// It fails closed on a missing/invalid schema marker, a duplicate or empty id,
// an empty argv, a missing binary hash, or an unsupported pass_when — an operator
// cannot author a malformed or silently-unenforced check.
func LoadAllowlist(path string) (*Allowlist, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // operator-curated, git-tracked path supplied by the lane packet
	if err != nil {
		return nil, fmt.Errorf("read verifier allowlist %q: %w", path, err)
	}
	return ParseAllowlist(raw)
}

// ParseAllowlist validates an allowlist document already read into memory (the
// pure, side-effect-free half of LoadAllowlist, used by the tests).
func ParseAllowlist(raw []byte) (*Allowlist, error) {
	var file AllowlistFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("parse verifier allowlist: %w", err)
	}
	if strings.TrimSpace(file.SchemaVersion) != AllowlistSchemaVersion {
		return nil, fmt.Errorf("verifier allowlist schema_version must be %q, got %q", AllowlistSchemaVersion, file.SchemaVersion)
	}
	if len(file.Checks) == 0 {
		return nil, fmt.Errorf("verifier allowlist must declare at least one check")
	}
	entries := make(map[string]AllowlistEntry, len(file.Checks))
	for idx, entry := range file.Checks {
		id := strings.TrimSpace(entry.ID)
		if id == "" {
			return nil, fmt.Errorf("verifier allowlist checks[%d].id must be a non-empty string", idx)
		}
		if _, exists := entries[id]; exists {
			return nil, fmt.Errorf("verifier allowlist checks[%d].id %q is duplicated", idx, id)
		}
		if len(entry.Argv) == 0 || strings.TrimSpace(entry.Argv[0]) == "" {
			return nil, fmt.Errorf("verifier allowlist check %q must declare a non-empty argv", id)
		}
		if strings.TrimSpace(entry.BinarySHA256) == "" {
			return nil, fmt.Errorf("verifier allowlist check %q must pin a binary_sha256", id)
		}
		passWhen := strings.TrimSpace(entry.PassWhen)
		if passWhen == "" {
			passWhen = passWhenExitZero
		}
		if passWhen != passWhenExitZero {
			return nil, fmt.Errorf("verifier allowlist check %q has unsupported pass_when %q (only %q is honored in this slice)", id, passWhen, passWhenExitZero)
		}
		entry.ID = id
		entry.PassWhen = passWhen
		entry.BinarySHA256 = strings.ToLower(strings.TrimSpace(entry.BinarySHA256))
		entries[id] = entry
	}
	return &Allowlist{entries: entries}, nil
}

// IDs returns the sorted set of allowlisted check ids (for diagnostics).
func (a *Allowlist) IDs() []string {
	ids := make([]string, 0, len(a.entries))
	for id := range a.entries {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Resolve returns the allowlist entry for a named check, refusing a check that
// is not curated. This is the "a lane NAMES but never AUTHORS the executed
// bytes" rule: an unknown id is rejected before any binary is touched.
func (a *Allowlist) Resolve(checkID string) (AllowlistEntry, error) {
	entry, ok := a.entries[strings.TrimSpace(checkID)]
	if !ok {
		return AllowlistEntry{}, fmt.Errorf("check %q is not in the operator-curated verifier allowlist (allowed: %s); a workflow may NAME a check but never AUTHOR the executed bytes", checkID, strings.Join(a.IDs(), ", "))
	}
	return entry, nil
}

// VerifyBinary hashes the resolved binary on disk and compares it to the pinned
// allowlist hash. A mismatch (a swapped or tampered binary) is refused: content
// addressing means the operator curated THESE bytes. Returns the actual hash on
// success so the receipt can record exactly what ran.
func VerifyBinary(entry AllowlistEntry) (string, error) {
	binary := entry.Argv[0]
	data, err := os.ReadFile(binary) //nolint:gosec // path is the operator-pinned allowlist binary, hash-checked immediately below
	if err != nil {
		return "", fmt.Errorf("resolve check %q binary %q: %w", entry.ID, binary, err)
	}
	sum := sha256.Sum256(data)
	actual := hex.EncodeToString(sum[:])
	if actual != entry.BinarySHA256 {
		return "", fmt.Errorf("check %q binary %q sha256 %s does not match the operator-pinned allowlist hash %s (the curated bytes drifted; refuse rather than run unverified bytes)", entry.ID, binary, actual, entry.BinarySHA256)
	}
	return actual, nil
}
