# Review: RFC 0114 draft (cross-family, independent)

You are the **reviewer**, on a different model family from the author. Falsify
the design where you can.

## Read first

- The RFC draft artifact (`.../artifacts/RFC_DRAFT.md`).
- `docs/rfcs/0113-runtime-read-scope-least-privilege.md`.
- `docs/dogfoods/rfc0114-read-scope/CONTEXT.md`.

You may open the cited source files to verify claims against reality.

## What to check (issue needs_revision if any FAIL)

1. **Ownership constraint correctness.** `principals`, `principal_clients`,
   `client_sessions` are owned by the runtime role `striatumd_rw`. Does the RFC
   correctly recognize that a plain `REVOKE SELECT FROM striatumd_rw` on a
   table the runtime role OWNS does NOT lock it out, and does it resolve this
   (e.g. ALTER OWNER + SECURITY DEFINER projection)? If the RFC assumes the
   0005 clients pattern works unchanged without addressing ownership, that is a
   needs_revision.
2. **Owner-bundle plan is concrete.** Bundle 0006 contents named:
   ALTER OWNER (if chosen), SECURITY DEFINER projection with
   `assert_daemon_authority()`, REVOKE/GRANT, `schema_authority` stamp,
   `LatestOwnerBundleVersion -> 6`, `ownerBundleLabels[6]`,
   `ReassertReadRevokes` extension.
3. **Parity path.** Projection-preferred + direct-fallback (on `42883`) dual
   path is preserved so an un-adopted DB still reads. Named handler(s)
   (`admin.ListPrincipals` etc.) and the same-DTO parity guarantee.
4. **Guard tests named and plausible** against
   `go/pkg/db/read_authority_inventory_pg_test.go` patterns.
5. **Doctor posture transition** to `partial_projection_gated`, computed (not
   hard-coded), `private_read_denial` stays false.
6. **Design-only discipline.** The RFC must NOT claim any live schema change,
   owner-bundle apply, or daemon restart happened in this run. Rollout is
   owner-applied out-of-band as a later step.
7. **Local-first / non-goal compliance** (no hosted service, no broad
   REVOKE that breaks reads, no standing broad read role as the final answer).

## Output

Write a single review-only finding at the declared path with one of the
supported verdicts:
- `accept` — implementable and correct.
- `accept_with_findings` — sound, with non-blocking improvements listed.
- `needs_revision` — a load-bearing element (especially #1, #2, #5) is wrong or
  missing; list precisely what must change.

Match the `author_line` in your packet exactly in the finding front matter.
Record your verdict via the packet's review/verdict command. Do not edit the
draft or any other file.
