# Coordinator Role (Dogfood 063 — RFC 0053 Phase B schema rename)

author: coordinator-role-001

See dogfood-061's `roles/coordinator.md` for canonical shape.
Schema-bump-class — codex `threat_model` design review is
load-bearing (back-compat + migration safety). Cycle budget is 1.

**Key tension to flag at synth:** is `escalation` artifact-kind in
scope here (one big dogfood) or deferred to dogfood-064 (smaller,
cleaner)? Synth makes the call; reviewer bounces if scope creep
threatens the v1.2 bump.

**Post-landing:** decide whether this is a minor or major bump.
Proposed: **minor** with deprecation warnings on old names for one
full release cycle (~3 minors), then remove old names in the cycle
after that. This is the conservative path consistent with v1.55.0
RFC 0048 V1.5 patterns.
