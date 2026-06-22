package workflowauthoring

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// RFC 0128 P0 (D196): the single-repo run is the invariant — a run writes
// exactly its one registered target repository. This file is the validate-time
// cross-repo guardrail. It (a) REFUSES, with a structured error, a workflow
// whose declared lane write-scope reaches outside the registered repository
// root, and (b) WARNS when a free-text prompt field carries a path token or
// org/repo slug that names a foreign repository. It never silently narrows
// scope — the #280 failure was a cross-repo prompt that quietly dropped the
// repos a run could not write. P1+ (dispatch-time scope_violation terminal
// state, read-only artifact federation, decomposition ergonomics) and the
// deferred first-class multi-repo manifest are explicitly NOT implemented here.

// DeferredSecondaryReposManifestKey names the first-class multi-repo manifest
// that RFC 0128 explicitly DECLINES to honor (until a future product trigger).
// No code reads this key: it is recorded here only so the deferred surface has
// a name and the guard test (TestSecondaryReposManifestIsNotHonored) can assert
// that a workflow setting it gains NO cross-repo write capability — every lane
// write-scope is still confined to the single registered repository.
const DeferredSecondaryReposManifestKey = "secondary_repos"

// CrossRepoWriteScopeError is the structured refusal for a write-scope path
// that resolves outside the registered repository root. The CLI maps it to the
// exit-7 validate contract and surfaces the offending path.
type CrossRepoWriteScopeError struct {
	JobID         string
	OffendingPath string
}

func (e *CrossRepoWriteScopeError) Error() string {
	return CrossRepoWriteScopeRefusalMessage(e.JobID, e.OffendingPath)
}

// CrossRepoWriteScopeRefusalMessage is the actionable refusal text. It names
// the offending job + path and points at decomposition (the shipped cross-repo
// outcome) rather than scope widening.
func CrossRepoWriteScopeRefusalMessage(jobID, offendingPath string) string {
	return fmt.Sprintf(
		"job %q declares write_scope allowed_path %q, which resolves outside the run's registered repository root. "+
			"A Striatum run writes exactly one registered repository (RFC 0128 / D196); cross-repo reach is refused, never silently narrowed. "+
			"For genuine cross-repo work, decompose it into one coordinated single-repo run per repository instead of widening this scope.",
		jobID, offendingPath,
	)
}

// RefuseCrossRepoWriteScope scans every job's declared
// write_scope.allowed_paths and returns a *CrossRepoWriteScopeError for the
// first path that resolves outside the registered repository root (an absolute
// path or a parent-escaping relative path). Job order is deterministic (sorted
// by id) so the surfaced offending path is stable. Malformed entries are left
// to structural Validate (exit 8); this guard is panic-safe on the unvalidated
// workflow map so it can run BEFORE Validate and win the exit-code race.
func RefuseCrossRepoWriteScope(workflow map[string]any) error {
	for _, entry := range crossRepoJobEntries(workflow) {
		scope, ok := entry.job["write_scope"].(map[string]any)
		if !ok {
			continue
		}
		for _, p := range stringsFromSlice(scope["allowed_paths"]) {
			if pathReachesOutsideRepoRoot(p) {
				return &CrossRepoWriteScopeError{JobID: entry.id, OffendingPath: p}
			}
		}
	}
	return nil
}

// pathReachesOutsideRepoRoot reports whether a repo-relative write-scope path
// escapes the registered repository root. A path escapes when it is absolute
// (anchored outside the repo) or when, after normalization, it climbs above the
// root via a leading parent segment. An empty path is left to structural
// validation (it is malformed, not a cross-repo reach). Interior ".." that
// nets back inside the repo (e.g. "a/../b") does NOT escape; structural
// validation still rejects it as malformed at exit 8.
func pathReachesOutsideRepoRoot(p string) bool {
	trimmed := strings.TrimSpace(p)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "/") {
		return true
	}
	cleaned := filepath.ToSlash(filepath.Clean(trimmed))
	return cleaned == ".." || strings.HasPrefix(cleaned, "../")
}

// ForeignRepoReachWarnings scans each job's free-text prompt fields
// (task_prompt.inline, objective, title) for tokens that name a repository or
// path outside the registered target, returning non-fatal warning records.
// registeredRepo is the registered repository's name (the CLI passes the repo
// root's base name; the daemon would pass the registered slug). The matcher is
// deliberately conservative (GitHub owner/repo references, absolute paths,
// parent-escaping paths, and bare owner/repo slugs that are not in-repo paths)
// to keep the false-positive rate near zero on prose that cites in-repo paths.
// Deep scanning of file-referenced prompt bodies and broader slug heuristics
// are a deliberate P0 boundary, left to a later phase.
func ForeignRepoReachWarnings(workflow map[string]any, registeredRepo string) []map[string]any {
	registeredRepo = strings.TrimSpace(registeredRepo)
	warnings := []map[string]any{}
	seen := map[string]bool{}
	for _, entry := range crossRepoJobEntries(workflow) {
		for _, field := range promptFreeTextFields(entry.job) {
			for _, token := range tokenizePrompt(field.value) {
				normalized, ok := foreignRepoToken(token, registeredRepo)
				if !ok {
					continue
				}
				key := entry.id + "\x00" + field.name + "\x00" + normalized
				if seen[key] {
					continue
				}
				seen[key] = true
				warnings = append(warnings, map[string]any{
					"rule":     "foreign_repo_reach",
					"severity": "warning",
					"job_id":   entry.id,
					"field":    field.name,
					"token":    normalized,
					"message":  ForeignRepoReachMessage(entry.id, field.name, normalized),
				})
			}
		}
	}
	return warnings
}

// ForeignRepoReachMessage is the non-fatal warning text surfaced for a foreign
// repository reference in a prompt field.
func ForeignRepoReachMessage(jobID, field, token string) string {
	return fmt.Sprintf(
		"job %q prompt field %s references %q, which names a repository or path outside the registered target. "+
			"A Striatum run is single-repo (RFC 0128 / D196). If this is genuine cross-repo work, decompose it into "+
			"coordinated single-repo runs rather than reaching across repositories from one run.",
		jobID, field, token,
	)
}

type crossRepoJobEntry struct {
	id  string
	job map[string]any
}

// crossRepoJobEntries returns the workflow's jobs as (id, job) pairs sorted by
// id, skipping any malformed (non-object) entry so the guard is panic-safe on
// an unvalidated workflow map.
func crossRepoJobEntries(workflow map[string]any) []crossRepoJobEntry {
	entries := []crossRepoJobEntry{}
	for _, item := range anySlice(workflow["jobs"]) {
		job, ok := item.(map[string]any)
		if !ok {
			continue
		}
		entries = append(entries, crossRepoJobEntry{id: stringValue(job["id"]), job: job})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].id < entries[j].id })
	return entries
}

type promptTextField struct {
	name  string
	value string
}

// promptFreeTextFields returns the free-text fields of a job that an operator
// authors as prose: the inline prompt body and the human-readable
// objective/title. Path-referenced prompt bodies are intentionally out of scope
// for P0.
func promptFreeTextFields(job map[string]any) []promptTextField {
	fields := []promptTextField{}
	if tp, ok := job["task_prompt"].(map[string]any); ok {
		if inline := strings.TrimSpace(stringValue(tp["inline"])); inline != "" {
			fields = append(fields, promptTextField{name: "task_prompt.inline", value: inline})
		}
	}
	if objective := strings.TrimSpace(stringValue(job["objective"])); objective != "" {
		fields = append(fields, promptTextField{name: "objective", value: objective})
	}
	if title := strings.TrimSpace(stringValue(job["title"])); title != "" {
		fields = append(fields, promptTextField{name: "title", value: title})
	}
	return fields
}

var promptTokenSplit = regexp.MustCompile("[\\s`\"'()\\[\\]{}<>,;|]+")

// tokenizePrompt splits free text into candidate tokens, preserving the
// characters that make up paths and repo slugs ('/', ':', '.', '-', '_') while
// trimming surrounding sentence punctuation.
func tokenizePrompt(text string) []string {
	raw := promptTokenSplit.Split(text, -1)
	out := make([]string, 0, len(raw))
	for _, token := range raw {
		// Trim only TRAILING sentence punctuation; a leading ".." is meaningful
		// (it marks a parent-escaping path) and must be preserved.
		token = strings.TrimRight(token, ".,:;")
		if token != "" {
			out = append(out, token)
		}
	}
	return out
}

var (
	githubRefPattern = regexp.MustCompile(`^(?:https?://)?(?:git@)?github\.com[:/]([A-Za-z0-9][A-Za-z0-9._-]*)/([A-Za-z0-9][A-Za-z0-9._-]+?)(?:\.git)?(?:[/#?].*)?$`)
	bareSlugPattern  = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9._-]*)/([A-Za-z0-9][A-Za-z0-9._-]*)$`)
)

// inRepoTopDirs are repository-relative top-level directory names that a bare
// two-segment token is far more likely to reference than a foreign repository;
// excluding them keeps the warn's false-positive rate near zero on prose that
// cites in-repo paths (e.g. "go/pkg", "docs/rfcs", "roles/author").
var inRepoTopDirs = map[string]bool{
	"go": true, "docs": true, "prompts": true, "roles": true,
	"artifacts": true, "cmd": true, "pkg": true, "internal": true,
	"examples": true, "scripts": true, "test": true, "tests": true,
	"src": true, "lib": true, "api": true, "web": true, "ui": true,
	"contracts": true, "dogfoods": true,
}

// fileishExtensions are file extensions on the second slug segment that signal
// a token is an in-repo file path (e.g. "roles/author.md"), not a repo slug.
var fileishExtensions = map[string]bool{
	"md": true, "go": true, "json": true, "txt": true, "yaml": true,
	"yml": true, "sql": true, "sh": true, "py": true, "js": true,
	"ts": true, "toml": true, "cfg": true, "lock": true, "mod": true,
}

// foreignRepoToken classifies a single token. It returns the normalized
// offending reference and true when the token names a foreign repository or an
// out-of-repo path; otherwise it returns false.
func foreignRepoToken(token, registeredRepo string) (string, bool) {
	// Absolute filesystem path: anchored outside any repo-relative scope.
	if strings.HasPrefix(token, "/") && len(token) > 1 {
		return token, true
	}
	// Parent-escaping relative path.
	if token == ".." || strings.HasPrefix(token, "../") || strings.Contains(token, "/../") {
		return token, true
	}
	// GitHub owner/repo reference (http(s), ssh, or bare host form).
	if m := githubRefPattern.FindStringSubmatch(token); m != nil {
		repo := strings.TrimSuffix(m[2], ".git")
		if registeredRepo != "" && strings.EqualFold(repo, registeredRepo) {
			return "", false
		}
		return m[1] + "/" + repo, true
	}
	// Bare owner/repo slug — conservative: skip in-repo top dirs and file paths.
	if m := bareSlugPattern.FindStringSubmatch(token); m != nil {
		owner, repo := m[1], m[2]
		if inRepoTopDirs[strings.ToLower(owner)] || hasFileishExtension(repo) {
			return "", false
		}
		if registeredRepo != "" && strings.EqualFold(repo, registeredRepo) {
			return "", false
		}
		return owner + "/" + repo, true
	}
	return "", false
}

func hasFileishExtension(segment string) bool {
	idx := strings.LastIndex(segment, ".")
	if idx < 0 || idx == len(segment)-1 {
		return false
	}
	return fileishExtensions[strings.ToLower(segment[idx+1:])]
}
