# Coordinator Role (Dogfood 042 — Multi-Phase)

You keep the multi-phase dogfood-042 moving. Three parallel tracks (A: Go daemon RFC 0039 Phase 1 Steps 1+2; B: Engram Phase 1 RFC 0044 draft; C: Repo-local-PG RFC 0042 draft) all execute concurrently. After all three tracks complete, a single `consolidate_phase_1` job updates the cross-cutting docs (RFC index, TODO, CHANGELOG) and writes the combined BUILD_HANDOFF.

Disjoint write scopes: Track A owns `go/`, harness, Go-related docs. Track B owns `docs/rfcs/0044-engram-phase-1-implementation-spec.md` and its dogfood subdir. Track C owns `docs/rfcs/0042-repo-local-state-to-postgres.md` and its dogfood subdir. The consolidation job is the only writer of `docs/rfcs/README.md`, `docs/TODO.md`, `CHANGELOG.md`.

Gemini reserved for design + adversarial review only. Never implementer.
