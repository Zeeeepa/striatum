// Package records defines pure value objects for RFC 0171 record dockets.
//
// The package has no daemon, database, filesystem, or blob-store dependency. It
// accepts already-indexed artifact/record rows, normalizes them into a
// deterministic docket, validates the fail-closed invariants, and renders the
// docket as JSON or compact Markdown.
package records

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

const (
	SchemaVersion = "striatum.records.docket.v1"

	PlacementBlobExhaust        = "blob_exhaust"
	PlacementGitPublication     = "git_publication"
	PlacementGitPointerManifest = "git_pointer_manifest"
)

// Docket is the RFC 0171 value object that bridges git-reviewable pointers and
// blob-resident generated records. GeneratedAt is caller-controlled; this
// package never stamps wall-clock time into a docket.
type Docket struct {
	RunID       string  `json:"run_id,omitempty"`
	GeneratedAt string  `json:"generated_at,omitempty"`
	Entries     []Entry `json:"entries"`
}

// Entry is one artifact or generated record included in a docket.
//
// Exactly one of ArtifactID or RecordID is required. LogicalName is the
// artifact-facing name; SourcePath is the original or repo-relative source path
// for record-shaped bodies. Kind is the artifact kind; Class is the record
// class. BlobKey and RepoPath are placement pointers and are validated according
// to Placement.
type Entry struct {
	RunID          string `json:"run_id,omitempty"`
	ArtifactID     string `json:"artifact_id,omitempty"`
	RecordID       string `json:"record_id,omitempty"`
	JobID          string `json:"job_id,omitempty"`
	LogicalName    string `json:"logical_name,omitempty"`
	SourcePath     string `json:"source_path,omitempty"`
	Kind           string `json:"kind,omitempty"`
	Class          string `json:"class,omitempty"`
	Placement      string `json:"placement"`
	RetentionClass string `json:"retention_class"`
	ContentSHA256  string `json:"content_sha256"`
	BlobKey        string `json:"blob_key,omitempty"`
	RepoPath       string `json:"repo_path,omitempty"`
	ContentType    string `json:"content_type"`
	SizeBytes      int64  `json:"size_bytes"`
	URI            string `json:"uri"`
}

// RenderedDocket is the stable JSON shape returned by RenderJSON.
type RenderedDocket struct {
	SchemaVersion string  `json:"schema_version"`
	RunID         string  `json:"run_id,omitempty"`
	GeneratedAt   string  `json:"generated_at,omitempty"`
	MerkleRoot    string  `json:"merkle_root"`
	Entries       []Entry `json:"entries"`
}

// Normalize returns a copy of d with trimmed scalar fields, lower-cased
// canonical enum/hash fields, and entries sorted by stable identity.
func (d Docket) Normalize() Docket {
	out := Docket{
		RunID:       strings.TrimSpace(d.RunID),
		GeneratedAt: strings.TrimSpace(d.GeneratedAt),
		Entries:     make([]Entry, len(d.Entries)),
	}
	for i, entry := range d.Entries {
		out.Entries[i] = normalizeEntry(entry, out.RunID)
	}
	sort.SliceStable(out.Entries, func(i, j int) bool {
		return entrySortKey(out.Entries[i]) < entrySortKey(out.Entries[j])
	})
	return out
}

// Validate checks the fail-closed docket invariants.
func (d Docket) Validate() error {
	normalized := d.Normalize()
	var problems []Problem
	if len(normalized.Entries) == 0 {
		problems = append(problems, Problem{Field: "entries", Message: "must include at least one entry"})
	}
	seen := map[string]int{}
	for i, entry := range normalized.Entries {
		prefix := fmt.Sprintf("entries[%d]", i)
		problems = append(problems, validateEntry(prefix, entry)...)
		key := entryIdentityKey(entry)
		if key != "" {
			if first, ok := seen[key]; ok {
				problems = append(problems, Problem{
					Field:   prefix,
					Message: fmt.Sprintf("duplicates identity from entries[%d]", first),
				})
			} else {
				seen[key] = i
			}
		}
	}
	if len(problems) > 0 {
		return ValidationError{Problems: problems}
	}
	return nil
}

// MerkleRoot returns the deterministic Merkle root over normalized entries.
func (d Docket) MerkleRoot() (string, error) {
	normalized := d.Normalize()
	if err := normalized.Validate(); err != nil {
		return "", err
	}
	leaves := make([]string, len(normalized.Entries))
	for i, entry := range normalized.Entries {
		canonical, err := canonicalEntryJSON(entry)
		if err != nil {
			return "", err
		}
		leaves[i] = sha256Hex([]byte("striatum.records.leaf.v1\n" + canonical))
	}
	return merkleRoot(leaves), nil
}

// RenderJSON renders the normalized docket with a computed Merkle root.
func (d Docket) RenderJSON() ([]byte, error) {
	normalized := d.Normalize()
	root, err := normalized.MerkleRoot()
	if err != nil {
		return nil, err
	}
	rendered := RenderedDocket{
		SchemaVersion: SchemaVersion,
		RunID:         normalized.RunID,
		GeneratedAt:   normalized.GeneratedAt,
		MerkleRoot:    root,
		Entries:       normalized.Entries,
	}
	return json.MarshalIndent(rendered, "", "  ")
}

func normalizeEntry(entry Entry, docketRunID string) Entry {
	out := Entry{
		RunID:          strings.TrimSpace(entry.RunID),
		ArtifactID:     strings.TrimSpace(entry.ArtifactID),
		RecordID:       strings.TrimSpace(entry.RecordID),
		JobID:          strings.TrimSpace(entry.JobID),
		LogicalName:    strings.TrimSpace(entry.LogicalName),
		SourcePath:     strings.TrimSpace(entry.SourcePath),
		Kind:           strings.TrimSpace(entry.Kind),
		Class:          strings.TrimSpace(entry.Class),
		Placement:      strings.ToLower(strings.TrimSpace(entry.Placement)),
		RetentionClass: strings.ToLower(strings.TrimSpace(entry.RetentionClass)),
		ContentSHA256:  strings.ToLower(strings.TrimSpace(entry.ContentSHA256)),
		BlobKey:        strings.TrimSpace(entry.BlobKey),
		RepoPath:       strings.TrimSpace(entry.RepoPath),
		ContentType:    strings.ToLower(strings.TrimSpace(entry.ContentType)),
		SizeBytes:      entry.SizeBytes,
		URI:            strings.TrimSpace(entry.URI),
	}
	if out.RunID == "" {
		out.RunID = docketRunID
	}
	return out
}

func validateEntry(prefix string, entry Entry) []Problem {
	var problems []Problem
	if (entry.ArtifactID == "") == (entry.RecordID == "") {
		problems = append(problems, Problem{Field: prefix + ".identity", Message: "must set exactly one of artifact_id or record_id"})
	}
	if entry.LogicalName == "" && entry.SourcePath == "" {
		problems = append(problems, Problem{Field: prefix + ".logical_name", Message: "must set logical_name or source_path"})
	}
	if entry.ArtifactID != "" && entry.Kind == "" {
		problems = append(problems, Problem{Field: prefix + ".kind", Message: "artifact entries must set kind"})
	}
	if entry.RecordID != "" && entry.Class == "" {
		problems = append(problems, Problem{Field: prefix + ".class", Message: "record entries must set class"})
	}
	if !isAllowedPlacement(entry.Placement) {
		problems = append(problems, Problem{Field: prefix + ".placement", Message: "must be blob_exhaust, git_publication, or git_pointer_manifest"})
	}
	if entry.RetentionClass == "" {
		problems = append(problems, Problem{Field: prefix + ".retention_class", Message: "must be non-empty"})
	}
	if !isSHA256Hex(entry.ContentSHA256) {
		problems = append(problems, Problem{Field: prefix + ".content_sha256", Message: "must be a 64-character sha256 hex digest"})
	}
	if entry.ContentType == "" {
		problems = append(problems, Problem{Field: prefix + ".content_type", Message: "must be non-empty"})
	}
	if entry.SizeBytes < 0 {
		problems = append(problems, Problem{Field: prefix + ".size_bytes", Message: "must be non-negative"})
	}
	if entry.URI == "" {
		problems = append(problems, Problem{Field: prefix + ".uri", Message: "must be non-empty"})
	} else if err := validateURI(entry); err != nil {
		problems = append(problems, Problem{Field: prefix + ".uri", Message: err.Error()})
	}
	if entry.BlobKey == "" && entry.RepoPath == "" {
		problems = append(problems, Problem{Field: prefix + ".pointer", Message: "must set blob_key or repo_path"})
	}
	switch entry.Placement {
	case PlacementBlobExhaust:
		if entry.BlobKey == "" {
			problems = append(problems, Problem{Field: prefix + ".blob_key", Message: "blob_exhaust entries must set blob_key"})
		}
	case PlacementGitPublication, PlacementGitPointerManifest:
		if entry.RepoPath == "" {
			problems = append(problems, Problem{Field: prefix + ".repo_path", Message: entry.Placement + " entries must set repo_path"})
		}
	}
	return problems
}

func isAllowedPlacement(value string) bool {
	switch value {
	case PlacementBlobExhaust, PlacementGitPublication, PlacementGitPointerManifest:
		return true
	default:
		return false
	}
}

func isSHA256Hex(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validateURI(entry Entry) error {
	parsed, err := url.Parse(entry.URI)
	if err != nil {
		return fmt.Errorf("must be a valid striatum URI")
	}
	if parsed.Scheme != "striatum" || parsed.Host == "" || parsed.Path == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("must be a striatum://artifact/<id>, striatum://record/<id>, or striatum://run/<id> URI")
	}
	id := strings.TrimPrefix(parsed.Path, "/")
	if id == "" || strings.Contains(id, "/") {
		return fmt.Errorf("must name exactly one virtual record id")
	}
	switch parsed.Host {
	case "artifact":
		if entry.ArtifactID == "" || id != entry.ArtifactID {
			return fmt.Errorf("artifact URI must match artifact_id")
		}
	case "record":
		if entry.RecordID == "" || id != entry.RecordID {
			return fmt.Errorf("record URI must match record_id")
		}
	case "run":
		if entry.RunID == "" || id != entry.RunID {
			return fmt.Errorf("run URI must match run_id")
		}
	default:
		return fmt.Errorf("must use artifact, record, or run URI host")
	}
	return nil
}

func entrySortKey(entry Entry) string {
	return strings.Join([]string{
		entry.RunID,
		entryIdentityKey(entry),
		entry.JobID,
		entry.LogicalName,
		entry.SourcePath,
		entry.Kind,
		entry.Class,
		entry.Placement,
		entry.RetentionClass,
		entry.ContentSHA256,
		entry.URI,
	}, "\x00")
}

func entryIdentityKey(entry Entry) string {
	if entry.ArtifactID != "" {
		return "artifact:" + entry.ArtifactID
	}
	if entry.RecordID != "" {
		return "record:" + entry.RecordID
	}
	return ""
}

func canonicalEntryJSON(entry Entry) (string, error) {
	raw, err := json.Marshal(entry)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func merkleRoot(leaves []string) string {
	level := append([]string(nil), leaves...)
	for len(level) > 1 {
		next := make([]string, 0, (len(level)+1)/2)
		for i := 0; i < len(level); i += 2 {
			left := level[i]
			right := left
			if i+1 < len(level) {
				right = level[i+1]
			}
			next = append(next, sha256Hex([]byte("striatum.records.node.v1\n"+left+"\n"+right)))
		}
		level = next
	}
	if len(level) == 0 {
		return sha256Hex([]byte("striatum.records.empty.v1\n"))
	}
	return level[0]
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
