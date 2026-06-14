package mutations

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/reads"
)

const defaultRecallDigestTimeout = 1500 * time.Millisecond

type RecallDigestOptions struct {
	Enabled bool
	Limit   int
	Timeout time.Duration
}

type recallDigestStatus struct {
	Status   string `json:"status"`
	Path     string `json:"path,omitempty"`
	HitCount int    `json:"hit_count,omitempty"`
	Error    string `json:"error,omitempty"`
}

func normalizeRecallDigestOptions(options RecallDigestOptions) RecallDigestOptions {
	if options.Limit <= 0 {
		options.Limit = reads.RecallDefaultLimit
	}
	if options.Limit > reads.RecallLimitCap {
		options.Limit = reads.RecallLimitCap
	}
	if options.Timeout <= 0 {
		options.Timeout = defaultRecallDigestTimeout
	}
	return options
}

func maybeRenderRecallShelf(ctx context.Context, runner db.Runner, options RecallDigestOptions, repositoryID string, inputs worktreeCreateInputs, worktreeRoot string) recallDigestStatus {
	options = normalizeRecallDigestOptions(options)
	shelfRel := filepath.ToSlash(filepath.Join(worktreeStateDir, "memory", "relevant.md"))
	if !options.Enabled {
		return recallDigestStatus{Status: "skipped"}
	}
	query := buildRecallDigestQuery(inputs)
	readCtx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	hits, meta, err := reads.RecallMemory(readCtx, runner, repositoryID, query, options.Limit)
	errorCode := ""
	if err != nil {
		log.Printf("recall digest skipped for worktree: %v", err)
		errorCode = "recall_failed"
		if meta.DegradedReason == "" {
			meta.DegradedReason = errorCode
		}
		hits = nil
	}
	body := renderRecallShelf(repositoryID, hits, meta)
	target := filepath.Join(worktreeRoot, filepath.FromSlash(shelfRel))
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		log.Printf("recall digest mkdir failed for %s: %v", target, err)
		return recallDigestStatus{Status: "failed", Path: shelfRel, Error: "mkdir_failed"}
	}
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		log.Printf("recall digest write failed for %s: %v", target, err)
		return recallDigestStatus{Status: "failed", Path: shelfRel, Error: "write_failed"}
	}
	status := "rendered"
	if len(hits) == 0 {
		status = "empty"
	}
	out := recallDigestStatus{Status: status, Path: shelfRel, HitCount: len(hits)}
	if errorCode != "" {
		out.Error = errorCode
	}
	return out
}

func buildRecallDigestQuery(inputs worktreeCreateInputs) string {
	fields := []string{
		fmt.Sprint(inputs.Job["title"]),
		fmt.Sprint(inputs.Job["workflow_job_id"]),
		fmt.Sprint(inputs.Job["job_type"]),
		fmt.Sprint(inputs.Job["role_id"]),
	}
	if expected, ok := inputs.Job["expected_artifacts_json"].([]any); ok {
		for _, item := range expected {
			if artifact, ok := item.(map[string]any); ok {
				fields = append(fields,
					fmt.Sprint(artifact["logical_name"]),
					fmt.Sprint(artifact["kind"]),
					fmt.Sprint(artifact["path"]),
				)
			}
		}
	}
	if docs, ok := inputs.Workflow["context_docs"].([]any); ok {
		for _, item := range docs {
			if doc, ok := item.(map[string]any); ok {
				fields = append(fields, fmt.Sprint(doc["path"]))
			}
		}
	}
	return compactJoin(fields)
}

func compactJoin(values []string) string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || value == "<nil>" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return strings.Join(out, " ")
}

func renderRecallShelf(repositoryID string, hits []reads.RecallHit, meta reads.RecallMeta) string {
	var b strings.Builder
	b.WriteString("# Striatum Recall Shelf\n\n")
	b.WriteString("This file is inert scaffold context generated from Striatum artifact metadata.\n\n")
	b.WriteString("## Provenance\n\n")
	_, _ = fmt.Fprintf(&b, "- repository_id: `%s`\n", markdownInline(repositoryID))
	_, _ = fmt.Fprintf(&b, "- query: `%s`\n", markdownInline(meta.Query))
	_, _ = fmt.Fprintf(&b, "- ranking_method: `%s`\n", markdownInline(meta.RankingMethod))
	_, _ = fmt.Fprintf(&b, "- source_surface: `%s`\n", markdownInline(meta.SourceSurface))
	b.WriteString("- redaction_tier: `metadata_only`\n")
	_, _ = fmt.Fprintf(&b, "- generated_at: `%s`\n", time.Now().UTC().Format(time.RFC3339))
	if meta.MaxIndexedCreatedAt != "" {
		_, _ = fmt.Fprintf(&b, "- max_indexed_created_at: `%s`\n", markdownInline(meta.MaxIndexedCreatedAt))
	}
	_, _ = fmt.Fprintf(&b, "- hit_count: `%d`\n", len(hits))
	if meta.DegradedReason != "" {
		_, _ = fmt.Fprintf(&b, "- degraded_reason: `%s`\n", markdownInline(meta.DegradedReason))
	}
	if len(hits) == 0 {
		b.WriteString("\nNo matching artifact metadata was found. State transitions do not depend on this shelf.\n")
		return b.String()
	}
	b.WriteString("\n## Hits\n\n")
	b.WriteString("| rank | kind | logical_name | path | run_id | published_at | score |\n")
	b.WriteString("| ---: | --- | --- | --- | --- | --- | ---: |\n")
	for index, hit := range hits {
		_, _ = fmt.Fprintf(&b, "| %d | %s | %s | %s | %s | %s | %.4f |\n",
			index+1,
			markdownCell(hit.ArtifactKind),
			markdownCell(hit.LogicalName),
			markdownCell(hit.RepoPath),
			markdownCell(hit.RunID),
			markdownCell(hit.CreatedAt),
			hit.Score,
		)
	}
	return b.String()
}

func markdownInline(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "`", "'"), "\n", " ")
}

func markdownCell(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.TrimSpace(value)
}

func stableKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
