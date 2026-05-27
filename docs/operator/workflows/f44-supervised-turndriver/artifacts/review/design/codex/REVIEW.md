---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["f44","design-review","threat_model"]
---

author: operator

# Threat-model review

Verdict: accept_with_findings.

The synthesis chooses the right broad shape: append-only supervised PATH augmentation is safer than absolute-argv0 rewriting, generator failure should report and exit non-fatally rather than crash/spin, and liveness needs both zombie-aware reads and start-token identity. I do not see a reason to reject the design.

Findings:

1. The `STRIATUM_SUPERVISED_PATH_DIRS` override needs an explicit trust-boundary requirement. It must be read only from the daemon/operator process environment, never from workflow JSON, work packets, artifacts, repository files, lane commands, or other target-repository input. It should be consumed to compute the single `PATH=` value, not emitted as its own `supervisedEnvEntries` variable. Tests should assert both that `supervisedEnvEntries` does not add `STRIATUM_SUPERVISED_PATH_DIRS` and that `ContentOnlyEnv` strips it from the generator child. This keeps the D145 topic+transcript-only boundary intact while still allowing PATH dirs.

2. The PATH-ordering invariant should be pinned more strongly. The synthesis says inherited daemon PATH stays first and local dirs are appended, which addresses system-tool shadowing, but the tests should assert ordering, not only membership. The same append-after-system rule must apply when `STRIATUM_SUPERVISED_PATH_DIRS` is set. Override dirs should also be filtered with the same absolute/existing/non-empty/non-relative constraints as the default local-bin dirs.

3. The graceful-failure contract should state the crash-safety ordering explicitly: `Loop.Run` may return `nil` only after `OnFailure` has synchronously and durably recorded the park/escalation. If the report cannot be committed, returning the reporting error as fatal is correct and must remain loud. Without commit-before-nil, a process exit could hide the failure the feature is meant to preserve.

4. The reaper design should be token-guarded on writes, not just reads. A `cmd.Wait` goroutine should record or log wait results only against the captured `pid_start_time`, should only transition in the terminal direction, and must not clobber a row already stopped/completed or replaced by a newer supervisor. Wait errors should be diagnostic, not panics or blind state rewrites. This keeps the reaper from turning PID reuse or a racing stop path into wrong liveness state.

Interrogation used: 2 rounds. Round 1 asked the live synthesizer about PATH override trust boundaries, non-forwarding, and ordering; the answer agreed these should become explicit requirements and tests. Round 2 asked about durable report-before-clean-exit and reaper write safety; the answer agreed on commit-before-nil and compare-and-set/start-token guarded reaper behavior. I stopped after round 2 because those answers covered the requested threat-model surfaces: PATH injection/wrong-binary risk, D145 boundary preservation, and graceful-failure crash safety.
