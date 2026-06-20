package metrics

// RFC 0137 Phase C — boot-time metrics_allowlist hash check (deliverable #3).
//
// The committed metrics_allowlist.json is the diff-reviewed manifest of every
// family the exporter may emit. It is embedded into the binary so the boot check
// is self-contained (no source tree at runtime). VerifyAllowlist recomputes the
// LIVE registry hash and compares it to the embedded manifest; any drift is a
// fatal boot abort BEFORE /metrics binds, and the same comparison fails the CI
// guardrail test (TestMetricsAllowlistMatchesRegistry). Adding a label therefore
// becomes a deliberate, reviewed manifest edit — mirroring the generated-route /
// error-catalog guardrail precedent.

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed metrics_allowlist.json
var embeddedAllowlist []byte

const allowlistVersion = 1

// Allowlist is the committed manifest: the SHA-256 of the canonical family set
// (the load-bearing field the boot check compares) plus the families themselves,
// committed alongside so a manifest edit is a reviewable diff.
type Allowlist struct {
	Version  int               `json:"version"`
	SHA256   string            `json:"sha256"`
	Families []AllowlistFamily `json:"families"`
}

// AllowlistFamily is one family entry in the committed manifest.
type AllowlistFamily struct {
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	Classification string   `json:"classification"`
	Labels         []string `json:"labels"`
}

// BuildAllowlist renders a registry as a committed manifest (sorted families,
// sorted labels, hash of the canonical form).
func BuildAllowlist(r *Registry) Allowlist {
	specs := r.Specs()
	fams := make([]AllowlistFamily, 0, len(specs))
	for _, f := range specs {
		labels := f.Labels
		if labels == nil {
			labels = []string{}
		}
		fams = append(fams, AllowlistFamily{
			Name:           f.Name,
			Type:           string(f.Type),
			Classification: string(f.Classification),
			Labels:         labels,
		})
	}
	return Allowlist{Version: allowlistVersion, SHA256: r.Hash(), Families: fams}
}

// MarshalAllowlist renders the manifest as the committed JSON bytes (indented,
// trailing newline) so STRIATUM_UPDATE_ALLOWLIST regeneration is a deterministic
// diff.
func MarshalAllowlist(a Allowlist) ([]byte, error) {
	b, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// LoadEmbeddedAllowlist parses the embedded committed manifest.
func LoadEmbeddedAllowlist() (Allowlist, error) {
	var a Allowlist
	if err := json.Unmarshal(embeddedAllowlist, &a); err != nil {
		return Allowlist{}, fmt.Errorf("parse embedded metrics_allowlist.json: %w", err)
	}
	return a, nil
}

// VerifyAllowlist is the boot-time guardrail: it recomputes the live default
// registry hash and compares it to the embedded manifest. A mismatch returns an
// error the daemon boot path treats as FATAL (hard boot abort) before the
// /metrics listener binds, so an un-reviewed family or label can never reach the
// wire.
func VerifyAllowlist() error {
	return verifyAllowlistAgainst(DefaultRegistry())
}

func verifyAllowlistAgainst(r *Registry) error {
	committed, err := LoadEmbeddedAllowlist()
	if err != nil {
		return err
	}
	live := r.Hash()
	if committed.SHA256 != live {
		return fmt.Errorf("metrics allowlist drift: committed metrics_allowlist.json sha256=%s but the live registry hashes to %s; a metric family or label changed — regenerate with STRIATUM_UPDATE_ALLOWLIST=1 go test ./pkg/metrics/... and review the diff", committed.SHA256, live)
	}
	return nil
}
