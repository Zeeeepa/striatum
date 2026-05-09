---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "info"
---

# RFC 0020 step 3 Design Review

author: reviewer-claude-opus-001

Date: 2026-05-09
Verdict: `accept`

## Pinned contracts (verified)

- **Daemon shape**: thin loop wrapping the existing
  `run_auto_sweep`. No new orchestration logic; just
  scheduling + signal handling + pidfile. ✓
- **Pidfile semantics**: `O_EXCL` open + alive-check on
  collision + `unlink` on stale. Race-safe enough for
  single-machine use; D020 keeps cross-machine out of scope. ✓
- **Signal handling**: `threading.Event` + `event.wait(...)`
  for the inter-sweep sleep. Re-entrant-safe handler. SIGTERM
  and SIGINT both trip it. ✓
- **JSONL output**: one envelope per sweep + final
  `watch_exit` line; flushed per line so consumers see them as
  they happen. ✓
- **Exit-on-terminal default**: matches the operational use
  case (overnight runs that finish before morning); opt-out
  available. ✓
- **CLI overrides match `recovery auto`**: substitutable from a
  cron / workflow perspective. ✓
- **No-policy regression**: workflow without `recovery_policy`
  continues to inherit defaults exactly as `recovery auto`
  does today. ✓
- **Test plan**: 8 cases cover max-sweeps, JSONL, pidfile
  collision + stale, exit-on-terminal both modes, SIGTERM
  shutdown, pidfile path. ✓
- **Version story**: 1.5.0 → 1.6.0 + RFC 0020 status drops
  the "step 3 deferred" qualifier. Cleaner than a 1.5.1 patch
  bump. ✓

## Notes

- The `policy` is resolved once at daemon startup, not per
  sweep. That's correct — re-resolving every iteration would
  let a workflow snapshot edit silently change behavior
  mid-flight. If an operator wants new policy, they kill the
  watcher and start a fresh one.
- The "stdout buffering" call-out in the research artifact is
  the right detail — `print(..., flush=True)` is enough; no
  need for `os.fsync` or unbuffered streams.
- Tests use the existing subprocess-spawn pattern from
  `test_service.py` and `test_web_ui.py`. No new fixture
  framework.

## Decision

`accept`. Step 3 is the smallest possible closure of RFC 0020:
a single loop body around `run_auto_sweep` plus
operational-grade signal handling. Closes the deferred slice
cleanly and makes the daemon use case feel native rather than
bolted on.
