# Review the GH #164 implementation (RFC 0114 / D173, owner bundle 0006)

You are the independent reviewer. The author's work packet, the upstream
`DRAFT.md` handoff artifact, and the implementation commits are your inputs.
Read `docs/rfcs/0114-read-scope-principals-sessions.md` and decision-log row
D173 first — the implementation must match the *accepted* design, not a
plausible alternative.

Verify, with evidence (run commands yourself; do not trust claims):

1. **Ownership-before-revoke.** `0006_identity_read_scope.sql` transfers
   `principals` / `principal_clients` / `client_sessions` to
   `CURRENT_USER` *before* any projection or revoke. A bundle that only
   REVOKEs from `striatumd_rw`-owned tables is a non-fix (the owner can
   self-re-grant) and is grounds for `needs_revision` on its own.
2. **Column-gate correctness vs live write paths.** `principal_clients`
   keeps `principal_id` denied but leaves the columns the runtime genuinely
   writes/filters on readable — grep the current `go/pkg/admin/tokens.go`
   (and any other consumer) yourself and check the gate against what the
   code reads *today*, not what the RFC snapshot said.
3. **`client_sessions` full deny is safe.** Verify zero runtime Go read
   consumers remain (search the whole `go/` tree).
4. **Doctor posture is derived, not asserted.** The
   `partial_projection_gated` flip must come from privilege/ownership
   probes + stamps; check the bundle-absent path still reports
   `broad_runtime_select`, and `private_read_denial` stays `false`.
5. **Fallback semantics.** With bundle 0006 NOT applied (production state
   until the operator applies it), every affected runtime read path must
   still work — confirm the projection-preferred/fallback discipline
   matches the bundle 0005 `PostgresAuthorizer` precedent.
6. **Tests are real and green.** Re-run the named pg-gated guard tests
   against live PostgreSQL via the `STRIATUM_PG_TEST_URL` env var in your
   lane environment, plus `make -C go test` for the touched packages and
   the CI lint. Paste actual output excerpts into your finding. Fabricated
   or unverifiable test claims are grounds for `reject`.

Record your finding at
`docs/operator/artifacts/rfc-0114-impl-164/review/REVIEW.md` with valid
finding front matter and `verdict_intent` one of
`accept` / `accept_with_findings` / `needs_revision` / `reject`, using the
byline supplied in your work packet. Submit the verdict through the review
verbs in your packet's `commands` block. Do not modify any file outside
your declared write scope.
