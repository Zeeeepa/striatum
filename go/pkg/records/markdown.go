package records

import (
	"fmt"
	"strings"
)

// RenderMarkdown renders the normalized docket in a compact reviewable form.
func (d Docket) RenderMarkdown() (string, error) {
	normalized := d.Normalize()
	root, err := normalized.MerkleRoot()
	if err != nil {
		return "", err
	}
	lines := []string{
		"# Striatum Record Docket",
		"",
	}
	if normalized.RunID != "" {
		lines = append(lines, fmt.Sprintf("- Run: `%s`", inlineCode(normalized.RunID)))
	}
	if normalized.GeneratedAt != "" {
		lines = append(lines, fmt.Sprintf("- Generated: `%s`", inlineCode(normalized.GeneratedAt)))
	}
	lines = append(lines, fmt.Sprintf("- Merkle root: `%s`", root), "")
	lines = append(lines,
		"| Identity | Job | Name/path | Kind/class | Placement | Retention | SHA-256 | Pointer | Size | URI |",
		"|---|---|---|---|---|---|---|---|---:|---|",
	)
	for _, entry := range normalized.Entries {
		cells := []string{
			tableCell(identityLabel(entry)),
			tableCell(codeOrDash(entry.JobID)),
			tableCell(codeOrDash(namePath(entry))),
			tableCell(codeOrDash(kindClass(entry))),
			tableCell(codeOrDash(entry.Placement)),
			tableCell(codeOrDash(entry.RetentionClass)),
			tableCell(codeOrDash(entry.ContentSHA256)),
			tableCell(codeOrDash(pointerLabel(entry))),
			fmt.Sprintf("%d", entry.SizeBytes),
			tableCell(codeOrDash(entry.URI)),
		}
		lines = append(lines, "| "+strings.Join(cells, " | ")+" |")
	}
	return strings.Join(lines, "\n") + "\n", nil
}

func identityLabel(entry Entry) string {
	if entry.ArtifactID != "" {
		return "`artifact:" + inlineCode(entry.ArtifactID) + "`"
	}
	return "`record:" + inlineCode(entry.RecordID) + "`"
}

func namePath(entry Entry) string {
	if entry.LogicalName != "" && entry.SourcePath != "" {
		return entry.LogicalName + " / " + entry.SourcePath
	}
	if entry.LogicalName != "" {
		return entry.LogicalName
	}
	return entry.SourcePath
}

func kindClass(entry Entry) string {
	if entry.Kind != "" && entry.Class != "" {
		return entry.Kind + " / " + entry.Class
	}
	if entry.Kind != "" {
		return entry.Kind
	}
	return entry.Class
}

func pointerLabel(entry Entry) string {
	if entry.BlobKey != "" && entry.RepoPath != "" {
		return "blob:" + entry.BlobKey + " / repo:" + entry.RepoPath
	}
	if entry.BlobKey != "" {
		return "blob:" + entry.BlobKey
	}
	return "repo:" + entry.RepoPath
}

func codeOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return "`" + inlineCode(value) + "`"
}

func tableCell(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "|", "\\|")
	return value
}

func inlineCode(value string) string {
	return strings.ReplaceAll(value, "`", "\\`")
}
