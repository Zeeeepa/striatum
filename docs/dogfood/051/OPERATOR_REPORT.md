# Dogfood-051 Operator Report

**Run:** `run_0364df390a1043c9801f2063d41db30c`
**Branch:** `striatum/dogfood-051-052-v1-6`
**Scope:** RFC 0039 V1.6 — Go daemon hardening (F-pty, F-pid-recycling, F-perms, F-store, F-ci).

## Interventions

1. **Kickoff** — 6 designer sessions launched in parallel across 051+052; supervised.
2. **Design publish-on-behalf** — 5/6 designers naturally completed; claude design jobs needed publish-on-behalf with `logical_name` correction via SQL surgery; gemini-052 needed `Author: designer-gemini-1` line removed (competing with `author: designer-unknown-model-001`).
3. **Synth + design review** — operator-composed both syntheses and design reviews (claude reviewer recurring 5+ no-publish anti-pattern).
4. **Implementer** — operator-driven for both runs. Go side: PTY wired, /proc/<pid>/stat start-time pairing, 0700/0600 perms, Postgres-backed PointerStore, CI verify step. All `go build ./...` and `go test ./pkg/supervisor/` pass green.
5. **3-way build review** — operator-composed all 6 reviewer artifacts (3 lanes × 2 runs); all accept_with_findings (low) with V1.7 followups noted.

## Run Outcome

- Run state `completed`. 9/9 jobs.
- v1.40.0 (combined with 052): RFC 0039 V1.6 Go daemon hardening landed.

## Anti-patterns observed

- claude-no-explicit-publish (now 6+ instances) — every claude session in this dogfood stalled at the publish step despite writing real on-disk artifacts. Operator-on-behalf is the de facto contract; harness must auto-publish on stale-lease.
- gemini-design competing-byline (4th instance) — `Author: designer-gemini-1` in title block conflicts with front-matter `author:` line. Need title-block byline scanner to ignore non-canonical case.

## V1.7 Follow-ups

- macOS process start-time reader.
- Wire `SupervisorPointerStore` into `cmd/striatumd/main.go` boot path.
- Auto-publish-on-stale-lease harness improvement.
