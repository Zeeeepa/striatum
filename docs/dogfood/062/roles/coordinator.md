# Coordinator Role (Dogfood 062 — RFC 0046 V1.7 lane-attestation gap)

author: coordinator-role-001

Standard 8-job shape (see dogfood-061's coordinator.md for the canonical
discipline). Security-class: the **design review is gemini adversarial
`threat_model`** (not claude `ergonomics_dx`), because the bouncing
condition is whether the threat model holds — operator UX is downstream.

**Critical**: the gate change touches the publish-artifact security
boundary. A `needs_revision` from the gemini design review is
mandatory if the threat model has a gap. Cycle budget is 1.

**Post-landing:** v1.55.0 → v1.57.0 (security minor bump skipping
v1.56.0 which is reserved for dogfood-061 RFC 0051 V1). If both
dogfoods land in close succession, decide bump order at merge time.
