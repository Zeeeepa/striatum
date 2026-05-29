# Phase 2 build — RFC 0088 closeout (gated on Phase 1 acceptance)

You are running only because all three Phase 1 reviews accepted. That
acceptance — produced by a codex owned-PTY agent-loop lane over tmux that
claimed a packet, implemented, published an attested artifact, and answered live
interrogations — IS the RFC 0088 codex P3 live-verify the prior session left
open. The owned-PTY agent-loop is now proven, so the irreversible deletions in
this job are safe to land.

Read:

- `docs/operator/workflows/rfc-0088-0089-closeout/TASK.md`
- `docs/rfcs/0088-deprecate-print-interactive-pty-lanes-agy-migration.md`
- `docs/operator/workflows/rfc-0088-p3-codex-verify/FINDINGS.md`
- `docs/decisions/decision-log.md` (entry format; the proposed D148-D151 are in
  the RFC's "Proposed decision-log entries" section)

## Tasks

1. **Apoptosis-clean deletion.** Remove the turn-driver
   (`go/pkg/agentloop/turn_driver.go` and the `striatumd -agent-loop
   -turn-driver` flag), the `single_shot` adapter capability, and the
   `--print`/`exec` supervised wrapper. Update or remove EVERY call site,
   export, catalog entry, installer profile, and test reference first — no
   dangling references, no test that passes while a real lane path is dead.
   Confirm the residual `gemini` / `gemini_cli` / `single_shot` references found
   in the working tree (catalog, liveness, conversation tests) are either
   migrated to `agy` or removed.
2. **Retired-vocabulary grep gate.** Add `docs/reference/retired-vocabulary.txt`
   (one token per line: `gemini_cli`, `gemini_default`, `single_shot`,
   `turn_driver`, `turn-driver`, and the retired `--print`/`exec` wrapper terms)
   and a build/test check (a Go test under an existing package, or a Make
   target) that fails if any listed token reappears in `go/` or `docs/` outside
   the `_archive/` tree. Read the allowlist from the file so future retirements
   are a doc edit, not a code edit.
3. **Decision-log entries D148-D151.** Draft them in
   `docs/decisions/decision-log.md` following the existing entry format. Each
   must reference the concrete evidence from THIS run: the Phase 1 codex
   live-verify session id and the run id, so a future auditor can reconstruct
   that the owned-PTY agent-loop was proven before the wrapper was deleted. Use
   the D148-D151 wording from the RFC's proposed-decision section as the basis.
4. **Docs + status flip.** Apply the RFC 0088 glossary delta to
   `docs/reference/ubiquitous-language.md` (add `agy`; mark `single_shot` and
   the turn-driver retired), update `docs/reference/spec.md` and
   `docs/reference/command-authority-matrix.md` to match the deleted surface,
   and flip RFC 0088 status from `proposed` to `accepted` in its header.

## Guardrails

- Stay strictly inside the write scope in your work packet.
- Deleting the `--print` wrapper does NOT affect this running lane — you are an
  owned-PTY agent-loop, not a `--print` wrapper — so the deletion is safe to
  perform now. Do not, however, weaken owned-PTY byline attestation.
- Add or update tests for every behavior change, including the new grep gate.

## Verify

```bash
cd go && gofmt -l . && go vet ./... && go test ./...
```

Run the new retired-vocabulary gate explicitly and show it passing.

## Publish

Write `docs/operator/workflows/rfc-0088-0089-closeout/artifacts/build_0088/HANDOFF.md` with:

- every file deleted and every call site updated, grouped so a reviewer can
  confirm no dangling references remain;
- the D148-D151 text landed and the exact evidence (session id + run id) each
  cites;
- the verification commands and results, including the grep-gate run;
- the live session id reviewers should interrogate.

Stay live for the Phase 2 interrogation panel after publishing and completing.
