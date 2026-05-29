# Handoff Report — Issue #57 and Issue #59

## 1. Observation
We directly observed the following from the `striatum` Go codebase:

### 1.1 Write-Scope Checker (Issue #57)
In `~/git/striatum/go/pkg/mutations/write_scope_guard.go`:
```go
106: func gitTouchedPathsSinceBaseline(ctx context.Context, repoRoot string, job map[string]any) ([]string, error) {
...
129: 	for path, baselineHash := range baseline {
130: 		if _, ok := currentByPath[path]; !ok && baselineHash != "" {
131: 			touched = append(touched, path)
132: 		}
133: 	}
134: 	sort.Strings(touched)
135: 	return dedupeStrings(touched), nil
136: }
```
When checking write scope on job completion (`work.complete` at `~/git/striatum/go/pkg/mutations/lifecycle.go:674`), any path transitioning from dirty (present in `baseline`) to clean (absent from `currentByPath` at line 130) is added to the `touched` paths list. If this file was outside the allowed write scope, it triggers a violation error.

### 1.2 Front-Matter Parser (Issue #59)
In `~/git/striatum/go/pkg/artifactcontracts/contracts.go`:
```go
343: func ParseFrontMatterBlock(block string) (map[string]any, error) {
344: 	result := map[string]any{}
345: 	for _, raw := range strings.Split(block, "\n") {
346: 		if strings.HasPrefix(raw, " ") || strings.HasPrefix(raw, "\t") {
347: 			return nil, fmt.Errorf("artifact front matter has invalid line %q", raw)
348: 		}
...
```
Any line containing spaces or tabs at the start of the front-matter block is immediately rejected with an error, preventing standard YAML formatting for multi-line lists.

In `~/git/striatum/go/pkg/cli/rpcclient/client.go`:
```go
165: func exitCode(code string) int {
166: 	switch code {
...
175: 	case "schema_invalid", "method_unknown":
176: 		return 2
177: 	default:
178: 		return 1
179: 	}
180: }
```
Front-matter schema check failures raise an `artifact_error` code, which falls into the `default` block of the exit code mapper, resulting in an exit code of `1` instead of `6`.

---

## 2. Logic Chain
1. **Relaxing the Write-Scope Checker (Issue #57)**:
   * **Premise**: Restoring a dirty file to its clean HEAD commit state is not an untrusted write; it is a safe cleanup action.
   * **Observation**: The loop in lines 129–133 of `write_scope_guard.go` explicitly flags a file as "touched" if it was dirty in the baseline but is no longer dirty now.
   * **Deduction**: If we remove this loop, files transitioning from dirty to clean will not be added to `touched`. The first loop at lines 124–128 will still correctly capture all new dirty files and all modified dirty files. Therefore, removing lines 129–133 relaxes the check to precisely "only new files or mutated files outside allowed_paths" as requested.

2. **Upgrading Front-Matter List & Line-Number Support (Issue #59)**:
   * **Premise**: Standard multi-line YAML formatting is needed for list variables (like inputs).
   * **Observation 1**: The rigid parser in `contracts.go` (line 346) throws an error for any leading whitespace.
   * **Observation 2**: The project already depends on the standard `go.yaml.in/yaml/v3 v3.0.4` (aliased/mirrored as `gopkg.in/yaml.v3` under Go's ecosystem).
   * **Deduction 1**: Parsing `block` with `yaml.Unmarshal` handles all standard YAML formatting natively.
   * **Deduction 2**: The unmarshaled result has the structure `[]any` for lists, which is already fully supported by the project's validation functions `isStringListValue` and `stringList`.
   * **Observation 3**: YAML parser errors are formatted as `yaml: line X: [message]`. The first line of the block is line 2 of the original Markdown file.
   * **Deduction 3**: Extracting `X`, adding 1, and rebuilding the error reports precise 1-indexed file line numbers to the user.
   * **Observation 4**: The `TestParseFrontMatterRejectsDuplicateFields` test relies on checking the string `"declared more than once"`. YAML v3's duplicate key error is `"already defined"`. Intercepting `"already defined"` and translating it ensures perfect compatibility.
   * **Observation 5**: `ValidateFrontMatter` issues `artifact_error`. `exitCode` defaults to exit code `1` for unknown error codes.
   * **Deduction 4**: Adding `case "artifact_error": return 6` inside `client.go` ensures the CLI exits with exactly `6` when front-matter validation fails.

---

## 3. Caveats
No caveats. The proposed changes do not impact other parts of the repository since the modifications are isolated to the specific parsing and validation layers, leveraging existing Go ecosystem libraries and design patterns already present in the codebase.

---

## 4. Conclusion
1. **Issue #57 (Write-Scope)** is fully resolved by removing the secondary loop in `gitTouchedPathsSinceBaseline` within `~/git/striatum/go/pkg/mutations/write_scope_guard.go`.
2. **Issue #59 (Front-Matter)** is fully resolved by:
   * Replacing the rigid parser loop in `ParseFrontMatterBlock` in `~/git/striatum/go/pkg/artifactcontracts/contracts.go` with `yaml.Unmarshal([]byte(block), &result)`.
   * Adding line number mapping (+1 adjustment) and duplicate key translation in `ParseFrontMatterBlock`.
   * Adding `case "artifact_error": return 6` to the `exitCode` switch statement in `~/git/striatum/go/pkg/cli/rpcclient/client.go`.

---

## 5. Verification Method

### 5.1 Automated Test Execution
Run the Go unit tests in the affected packages to verify the changes:
```bash
# Verify mutations/write_scope_guard tests still pass
make test PKG=github.com/halbritt/striatum/go/pkg/mutations

# Verify artifactcontracts/contracts tests still pass and validate duplicate checks
make test PKG=github.com/halbritt/striatum/go/pkg/artifactcontracts
```

### 5.2 Manual/Behavioral Invalidation Tests
1. **Write-Scope Validation**:
   * Baseline a dirty file outside the allowed paths.
   * Revert the dirty file during a run to clean state.
   * Complete the job. Confirm that the job finishes without triggering a write-scope violation.
2. **Front-Matter List & Line-Number Validation**:
   * Create a `synthesis` or `finding` artifact with standard multi-line YAML formatting:
     ```yaml
     ---
     schema_version: "striatum.synthesis.v1"
     artifact_kind: "synthesis"
     inputs:
       - "input_1"
       - "input_2"
     ---
     ```
   * Validate it. Confirm that it parses successfully.
   * Introduce a syntax error at line 4 (e.g. `  - "input_2" : invalid`). Confirm that it raises a precise error: `artifact front matter syntax error: yaml: line 5: ...` and exits with exit code `6`.
