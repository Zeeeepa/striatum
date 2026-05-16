# Coordinator Role (Dogfood 061 — RFC 0051 V1 auto-finalize)

author: coordinator-role-001

You drive dogfood-061 end to end. 9 jobs total, single implement track.
Shape:

1. **3 designs** — codex, claude, gemini in parallel. Each covers the
   full RFC 0051 V1 feature (reconciliation hook + per-session scan +
   atomic publish/verdict/complete + 2 event types + feature flag +
   refusal-path mapping + 4 acceptance tests).
2. **1 synthesis** — codex locks per-method file paths, function
   signatures, event-payload shape, feature-flag check point, test
   paths.
3. **1 design review** — claude `ergonomics_dx` gates implement.
   `max_iterations: 1` (058 lesson — no three-synth pattern).
4. **1 implementer** — codex single track. Sub-agents per cluster:
   hook + scan, internal call sequence, event types, feature flag.
5. **3-way build review** — codex `threat_model`, claude
   `ergonomics_dx`, gemini adversarial `threat_model`.

**No dual-track.** Auto-finalize is one cohesive feature. Reviewers
MUST bounce a dual-track synthesis.

**Operating mode**: v1.55.0 daemon-required. Postgres-live. No
`STRIATUM_DAEMON_REQUIRED=0` escape. The new V1.5 hardening should
mean wrapper-permission, capability-denial, audit-chain, role-grants,
and chain-anchor schema are all green pre-flight.

**Friction discipline.** Update `docs/HARNESS_FRICTION_PATTERNS.md`
incrementally during the run for every harness gotcha — gemini lane
unsupported flag, wrapper regeneration anomaly, claim-next refusal
without --force, anything. Per D091, write per-intervention, not at
end.

**Post-landing acceptance** (RFC 0051 §Acceptance):
1. STRIATUM_AUTO_FINALIZE_ENABLE=1 + agent writes a valid
   `finding.v1` to expected path → job auto-finalizes within the
   next lease tick without operator intervention.
2. Same scenario but byline mismatch → falls through to lane-stall
   blocker; operator override still works (RFC 0046 V1 path intact).
3. Same scenario but feature flag unset → no auto-finalize; lane-stall
   blocker fires as today.
4. End-to-end run on a representative workflow (e.g. examples/
   code-change-flow) records zero operator-on-behalf publishes
   when all agents wrote valid artifacts.

**Post-landing operator steps:**
1. Bump pyproject.toml 1.55.0 → 1.56.0.
2. CHANGELOG.md entry under v1.56.0.
3. ROADMAP.md §4.2 marked ✅ shipped.
4. RFC 0051 status → accepted.
5. Merge dogfood branch to main; tag v1.56.0; push.
