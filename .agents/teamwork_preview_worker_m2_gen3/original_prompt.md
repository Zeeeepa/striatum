## 2026-05-29T12:03:11Z

Objective:
Implement robust, verified fixes for the following four GitHub issues inside the Striatum repository according to the specifications in ~/git/striatum/.agents/orchestrator_gen3/synthesis.md:
1. Issue #57 (Write-Scope Strictness): Relax the git-based write-scope checker to ensure it only flags new files or mutated files outside allowed_paths as violations. Clean or stashed files (transitioning from dirty to clean compared to baseline) must not trigger a violation.
2. Issue #58 (Duplicate Artifact Publication in submit-review): Update review submission handlers so they do not crash with a raw unique key database constraint error if a finding artifact has already been published. Log a helpful, user-friendly message and proceed with recording the verdict.
3. Issue #59 (Strict Front-Matter List Formatting): Enhance front matter parsing in synthesis and finding artifacts to support standard multi-line YAML formatting for lists (such as inputs). Return precise syntax errors with line numbers rather than a silent exit-code 6.
4. Issue #60 (Rigid Session Lifetime Enforcement): Support a parameter or automated logic to replace duplicate active sessions on the same lane for a run, avoiding manual unregister blocks.

You MUST write corresponding regression tests in the codebase for each of these issues to ensure their correctness and verify they are fully resolved.

Scope boundaries:
Only modify the files relevant to the above issues:
- go/pkg/mutations/write_scope_guard.go (Issue #57)
- go/pkg/mutations/review.go (Issue #58)
- go/pkg/artifactcontracts/contracts.go (Issue #59)
- go/pkg/cli/rpcclient/client.go (Issue #59)
- go/pkg/mutations/lifecycle.go (Issue #60)
And any test files under these packages (or add new test files if needed).
Ensure compliance with AGENTS.md instructions (e.g. lowercase privacy-safe bylines: `author: <role-name>-<model-name>-<ordinal>` on new files, no retired vocabulary, etc.).
