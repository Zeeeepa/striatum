# Plan of Record (Gen 3)

## Milestone 1: Exploration & Triage
- [ ] Dispatch 3 parallel Explorers:
  - **Explorer 1**: Analyze Issue #57 (Write-Scope Strictness) and Issue #59 (Strict Front-Matter List Formatting)
  - **Explorer 2**: Analyze Issue #58 (Duplicate Artifact Publication) and Issue #60 (Rigid Session Lifetime Enforcement)
  - **Explorer 3**: Analyze Issues #49 & #54 (PTY Supervision, Rebridge, & Re-queueing)
- [ ] Synthesize findings into a concrete implementation strategy under `~/git/striatum/.agents/orchestrator_gen3/synthesis.md`

## Milestone 2: CLI, Session & Front-Matter Implementation
- [ ] Dispatch a Worker to implement fixes for:
  - Issue #57 (write scope git diff parsing relaxation)
  - Issue #58 (duplicate artifact publication duplicate key catching)
  - Issue #59 (detailed front matter list YAML formatting & syntax reporting with lines)
  - Issue #60 (replacing duplicate active sessions automatically)
- [ ] Implement regression tests for each issue in Milestone 2
- [ ] Dispatch Reviewers to audit and approve the implementation

## Milestone 3: PTY Supervision, Rebridge & Re-queueing Implementation
- [ ] Dispatch a Worker to implement fixes for:
  - Issue #49 (re-queued packet resuming after checkpoint resolution)
  - Issue #54 (supervision rebridge RFC 0089 Phase 2 and status details)
- [ ] Implement regression tests for each issue in Milestone 3
- [ ] Dispatch Reviewers to audit and approve the implementation

## Milestone 4: Dual-Track Verification & Forensic Audit
- [ ] Run the full Go test suite (`go test -race ./...`) and vet commands
- [ ] Run the retired vocabulary grep gate check
- [ ] Dispatch Forensic Auditor to verify integrity and compile a CLEAN verdict report
- [ ] Report final completion back to the Sentinel (caller)
