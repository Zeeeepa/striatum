# Task: commit the witness-cleared release

Only runs after the collaboration ledger records `accept`. Publish
`VERIFIED_RELEASE.md`: the durable record a downstream consumer will trust
without re-reading the code.

Restate every claim with the status its witness **earned** and the witness
inline:

| claim | earned status | witness |
|---|---|---|

Rules:
- No completion language survives above the witnessed status.
- `DESIGNED` rows stay — they are the honest backlog, and naming them is the
  point. A reader must be able to tell, per line, what runs from what was
  intended.

Follow your role definition (`roles/committer.md`).
