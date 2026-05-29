# Exploration Report: Issues #57 and #59 Analysis

## Executive Summary
This report presents a read-only codebase exploration and precise implementation recommendations for:
1. **Issue #57 (Write-Scope Strictness)**: Relaxing the git-based write-scope checker so that files transitioning from dirty to clean compared to their baseline state do not trigger write-scope violations.
2. **Issue #59 (Strict Front-Matter List Formatting)**: Upgrading the rigid custom front-matter parser to support standard multi-line YAML list formatting, translating parse errors to include precise original file line numbers, mapping duplicate key errors to preserve existing test compatibility, and assigning exit code `6` to front-matter validation (`artifact_error`) failures.

---

## 1. Issue #57 (Write-Scope Strictness)

### 1.1 Location & Triggering Mechanism
The git-based write-scope checker is implemented in:
* **File Path**: `~/git/striatum/go/pkg/mutations/write_scope_guard.go`
* **Target Function**: `gitTouchedPathsSinceBaseline(ctx context.Context, repoRoot string, job map[string]any) ([]string, error)` (Lines 106–136)

**Triggering Path**:
1. When a job is completed, `striatum complete` triggers the RPC command `work.complete`.
2. This is handled by `HandleComplete` in `~/git/striatum/go/pkg/mutations/lifecycle.go` (Line 650).
3. At Line 674, it executes the write-scope check:
   ```go
   if err := enforceWriteScopeClean(ctx, tx, repositoryID, job); err != nil {
       return nil, err
   }
   ```
4. `enforceWriteScopeClean` in `~/git/striatum/go/pkg/mutations/write_scope_guard.go` calls `gitTouchedPathsSinceBaseline(ctx, repoRoot, job)` to identify which paths were modified during the run.
5. If any returned paths are outside `allowed_paths` (and not in `ignored_paths`), a write-scope violation is raised, returning an `invalid_transition` RPC error and preventing job completion.

### 1.2 Issue Diagnosis
The bug resides inside `gitTouchedPathsSinceBaseline` (Lines 129–133):
```go
129: 	for path, baselineHash := range baseline {
130: 		if _, ok := currentByPath[path]; !ok && baselineHash != "" {
131: 			touched = append(touched, path)
132: 		}
133: 	}
```
* `baseline` represents files that were dirty (modified/untracked) at the time the job was claimed.
* `currentByPath` represents files currently dirty compared to the HEAD commit.
* If a file was dirty at the baseline claim (`baselineHash != ""`) but is cleaned up (reverted/restored to HEAD state) during the run, it disappears from `git status` and is thus absent from `currentByPath`.
* The secondary loop checks if `path` from `baseline` is absent from `currentByPath`. If so, it adds it to `touched`.
* Since this file is outside `allowed_paths`, it triggers a write-scope violation.
* This is incorrect because restoring a dirty file to its clean HEAD state is not a mutation/untrusted write; it is a transition from dirty to clean.

### 1.3 Recommended Solution
To relax the checker so that files transitioning from dirty to clean do not trigger a violation, **remove the second loop entirely**.

By only processing the `currentByPath` map (the first loop), we ensure that the only files considered "touched" are:
1. Newly dirty files (files currently dirty that were not dirty in the baseline: `!ok`).
2. Mutated dirty files (files currently dirty that were in the baseline but have a different hash: `baselineHash != currentHash`).

Any file that transitions from dirty to clean will not be in `currentByPath` and will be safely ignored, preventing incorrect write-scope violations.

#### Proposed Code Diff:
```go
// In ~/git/striatum/go/pkg/mutations/write_scope_guard.go:

func gitTouchedPathsSinceBaseline(ctx context.Context, repoRoot string, job map[string]any) ([]string, error) {
	current, err := gitChangedPathSnapshots(ctx, repoRoot)
	if err != nil {
		return nil, err
	}
	baseline := gitBaselineFromJob(job)
	if len(baseline) == 0 {
		paths := make([]string, 0, len(current))
		for _, item := range current {
			paths = append(paths, item.Path)
		}
		return paths, nil
	}
	currentByPath := map[string]string{}
	for _, item := range current {
		currentByPath[item.Path] = item.Hash
	}
	touched := []string{}
	for path, currentHash := range currentByPath {
		if baselineHash, ok := baseline[path]; !ok || baselineHash != currentHash {
			touched = append(touched, path)
		}
	}
-	for path, baselineHash := range baseline {
-		if _, ok := currentByPath[path]; !ok && baselineHash != "" {
-			touched = append(touched, path)
-		}
-	}
	sort.Strings(touched)
	return dedupeStrings(touched), nil
}
```

---

## 2. Issue #59 (Strict Front-Matter List Formatting)

### 2.1 Location & Current Parsing Logic
Front-matter parsing is implemented in:
* **File Path**: `~/git/striatum/go/pkg/artifactcontracts/contracts.go`
* **Target Function**: `ParseFrontMatterBlock(block string) (map[string]any, error)` (Lines 343–368)

**Current Logic**:
The custom parser reads the front-matter block line-by-line using standard string splitting and splitting on `:`. It explicitly rejects any line starting with a space or tab:
```go
346: 		if strings.HasPrefix(raw, " ") || strings.HasPrefix(raw, "\t") {
347: 			return nil, fmt.Errorf("artifact front matter has invalid line %q", raw)
348: 		}
```
This design makes it impossible to support standard multi-line YAML formatting for lists (e.g. `inputs:` followed by indented items prefixed with `-`), as any indented lines are rejected immediately.

### 2.2 Recommended Solution
We recommend rewriting `ParseFrontMatterBlock` to use the standard `gopkg.in/yaml.v3` YAML library, which is already present in the project's Go module dependencies (`go.yaml.in/yaml/v3 v3.0.4`).

#### 2.2.1 Standard Multi-line YAML List Support
Using `yaml.Unmarshal([]byte(block), &result)` handles standard multi-line YAML lists natively. The parsed list of strings will be loaded into `result` as `[]any` (containing string items).
The schema validation engine in `contracts.go` is already designed to process `[]any` lists containing strings through `isStringListValue` and `stringList`. Thus, no changes to the schema fields or checks are needed!

#### 2.2.2 Precise Line-Number Error Reporting
When `yaml.Unmarshal` encounters a syntax error, it returns an error formatted as: `yaml: line X: [reason]`.
Since the parsed `block` is extracted *excluding* the opening `---` line (which occupies line 1 of the file), line `X` in the block corresponds to line `X + 1` of the original file.
We can intercept this error, parse `X`, replace it with `X + 1`, and return a precise, user-friendly syntax error pointing directly to the offending line of the Markdown file.

#### 2.2.3 Compatibility with Duplicate Key Tests
The test `TestParseFrontMatterRejectsDuplicateFields` in `pkg/artifactcontracts/contracts_test.go` expects duplicate field errors to contain `"declared more than once"`.
Under `gopkg.in/yaml.v3`, a duplicate key error is formatted as: `yaml: line X: mapping key "K" already defined at line Y`.
We can detect `"already defined"`, extract the key name `"K"`, and map it to the expected `"declared more than once"` format, ensuring perfect backward compatibility.

#### 2.2.4 Correct Exit Code 6 Mapping
To map front-matter validation failures (which raise a `code = "artifact_error"`) to exit code `6`, add a case in the CLI client's exit code mapper:
* **File Path**: `~/git/striatum/go/pkg/cli/rpcclient/client.go`
* **Target Function**: `exitCode(code string) int` (Lines 165–180)

### 2.3 Proposed Code Implementation Details

#### Proposed change to `ParseFrontMatterBlock` in `pkg/artifactcontracts/contracts.go`:
```go
package artifactcontracts

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3" // Add YAML import
)

// ...

func ParseFrontMatterBlock(block string) (map[string]any, error) {
	result := map[string]any{}
	err := yaml.Unmarshal([]byte(block), &result)
	if err != nil {
		errMsg := err.Error()

		// 1. Map duplicate key error to the specific expected format to pass existing tests
		if strings.Contains(errMsg, "already defined") {
			if startIdx := strings.Index(errMsg, "mapping key \""); startIdx != -1 {
				keyStart := startIdx + len("mapping key \"")
				if keyEnd := strings.Index(errMsg[keyStart:], "\""); keyEnd != -1 {
					key := errMsg[keyStart : keyStart+keyEnd]
					return nil, fmt.Errorf("artifact front matter field %q is declared more than once", key)
				}
			}
		}

		// 2. Map block line number to 1-indexed original file line number (block starts at line 2)
		if idx := strings.Index(errMsg, "line "); idx != -1 {
			start := idx + 5
			end := start
			for end < len(errMsg) && errMsg[end] >= '0' && errMsg[end] <= '9' {
				end++
			}
			if end > start {
				lineNumStr := errMsg[start:end]
				if lineNum, pErr := strconv.Atoi(lineNumStr); pErr == nil {
					actualLineNum := lineNum + 1
					errMsg = errMsg[:start] + strconv.Itoa(actualLineNum) + errMsg[end:]
				}
			}
		}
		return nil, fmt.Errorf("artifact front matter syntax error: %s", errMsg)
	}
	return result, nil
}
```

#### Proposed change to `exitCode` in `pkg/cli/rpcclient/client.go`:
```go
func exitCode(code string) int {
	switch code {
	case "version_incompatible":
		return 10
	case "daemon_unreachable":
		return 11
	case "repo_not_registered":
		return 12
	case "token_missing", "token_malformed", "token_invalid", "token_revoked", "token_expired", "capability_missing", "capability_scope_mismatch", "capability_expired":
		return 13
	case "schema_invalid", "method_unknown":
		return 2
	case "artifact_error": // Front-matter validation and contract failures
		return 6
	default:
		return 1
	}
}
```
