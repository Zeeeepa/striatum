package reads

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// acknowledgedLossSchema is the schema string a curated baseline file must
// declare to be honored. A file with any other (or missing) schema is treated as
// a parse error and downgrades nothing — the safe-degrade default.
const acknowledgedLossSchema = "striatum.doctor.acknowledged_loss.v1"

// acknowledgedLossRelPath is the repository-relative path of the operator-curated
// baseline of reviewed, immaterial artifact losses (Rule C of D205). The file is
// optional: a missing file is the normal state and downgrades nothing. The live
// baseline is operator-curated provenance committed separately from this code; the
// reader only consumes it.
const acknowledgedLossRelPath = "docs/operator/doctor-acknowledged-loss.json"

// acknowledged-loss baseline load statuses, surfaced as the additive
// `acknowledged_loss_status` block field so verbose doctor consumers can see
// whether the curated baseline was absent, loaded, or unparseable.
const (
	acknowledgedLossAbsent     = "absent"
	acknowledgedLossLoaded     = "loaded"
	acknowledgedLossParseError = "parse_error"
)

// acknowledgedLossEntry is one curated, sha-bound acknowledgment that a specific
// artifact's recorded content is a known, reviewed, immaterial loss.
type acknowledgedLossEntry struct {
	ArtifactID     string `json:"artifact_id"`
	RepoPath       string `json:"repo_path"`
	ContentSHA256  string `json:"content_sha256"`
	Reason         string `json:"reason"`
	AcknowledgedBy string `json:"acknowledged_by"`
	AcknowledgedAt string `json:"acknowledged_at"`
}

// acknowledgedLossFile is the on-disk shape of the curated baseline.
type acknowledgedLossFile struct {
	Schema  string                  `json:"schema"`
	Entries []acknowledgedLossEntry `json:"entries"`
}

// acknowledgedLossSet is the per-repo lookup of curated loss acknowledgments,
// keyed by artifact_id. status records how loading went (absent/loaded/
// parse_error) so the doctor block can surface it without aborting the check.
type acknowledgedLossSet struct {
	status string
	byID   map[string]acknowledgedLossEntry
}

// loadAcknowledgedLossSet reads the curated baseline at
// <repoRoot>/docs/operator/doctor-acknowledged-loss.json. It NEVER errors: a
// missing file -> empty "absent" set (no downgrades); an unreadable/malformed
// file or wrong schema -> empty "parse_error" set (still no downgrades). This is
// the load-bearing safety default — a broken or missing baseline can never mask a
// genuine-loss problem, it can only fail to downgrade one.
func loadAcknowledgedLossSet(repoRoot string) acknowledgedLossSet {
	set := acknowledgedLossSet{status: acknowledgedLossAbsent, byID: map[string]acknowledgedLossEntry{}}
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		return set
	}
	path := filepath.Join(repoRoot, filepath.FromSlash(acknowledgedLossRelPath))
	body, err := os.ReadFile(path)
	if err != nil {
		// Missing (or otherwise unreadable) file is the normal state: no entries.
		return set
	}
	var file acknowledgedLossFile
	if err := json.Unmarshal(body, &file); err != nil {
		set.status = acknowledgedLossParseError
		return set
	}
	if strings.TrimSpace(file.Schema) != acknowledgedLossSchema {
		set.status = acknowledgedLossParseError
		return set
	}
	for _, entry := range file.Entries {
		id := strings.TrimSpace(entry.ArtifactID)
		if id == "" {
			continue
		}
		set.byID[id] = entry
	}
	set.status = acknowledgedLossLoaded
	return set
}

// honor reports whether the curated baseline acknowledges the loss of this exact
// artifact content. The match is sha-bound: an entry is honored only when its
// content_sha256 equals the row's recorded content_sha256, so a stale or wrong
// entry can never mask a *different* future problem at the same artifact id. An
// id match with a mismatched (or empty) sha is NOT honored — the loss stays a
// problem.
func (s acknowledgedLossSet) honor(artifactID, contentSHA string) (acknowledgedLossEntry, bool) {
	entry, ok := s.byID[strings.TrimSpace(artifactID)]
	if !ok {
		return acknowledgedLossEntry{}, false
	}
	entrySHA := strings.TrimSpace(entry.ContentSHA256)
	rowSHA := strings.TrimSpace(contentSHA)
	if entrySHA == "" || rowSHA == "" || !strings.EqualFold(entrySHA, rowSHA) {
		return acknowledgedLossEntry{}, false
	}
	return entry, true
}
