Run `striatum verifier run` against the sanctioned checks and publish the minted receipts. The builtin checks (builtin:go-test/vet/build, artifact-anchor-integrity) run with no operator JSON and cap their claims at ASSERTED; VERIFIED needs an external check the operator has pinned (`striatum verifier pin --host-here`) AND attested. Receipts come from the engine's exit codes, not the builder's prose. Do NOT edit verification/allowlist.intent.json — it is in your forbidden_paths. A check whose negative control passes voids the receipt: report it RED.

## For this run (RFC 0142 P0)

`verification/allowlist.intent.json` has **no external checks** (empty), so run
**only the builtins** that back the claim ledger. Run each against your worktree
(the P0 work is present):

Write all receipts under your write scope `docs/operator/artifacts/vg_rfc0142_p0/verify/`.
Your declared expected artifact is `RECEIPTS.md` (kind `receipt`, so it must be a
valid `receipt.v1` — a verifier `--out` file already is). Make `RECEIPTS.md` the
`go-test` receipt and write the other two as siblings:

```
R=docs/operator/artifacts/vg_rfc0142_p0/verify
striatum verifier run --check-id builtin:go-build --cwd "$PWD" --out "$R/receipt-go-build.json" --json
striatum verifier run --check-id builtin:go-vet   --cwd "$PWD" --out "$R/receipt-go-vet.json"   --json
striatum verifier run --check-id builtin:go-test  --cwd "$PWD" --out "$R/RECEIPTS.md"           --json
```

`--cwd "$PWD"` must be the **absolute worktree root** (the verifier finds the Go
module in `go/`). The PG-backed two-role suites skip without a cluster bound — that
is expected; `go-test` should still pass on the non-PG tests. Then publish
`RECEIPTS.md` as the `receipt` artifact (the two sibling receipt files live in the
same write-scope dir as supporting evidence the adjudicator reads). Report each
check's `passed` + `classification` (expect `asserted`) faithfully from the
receipts — never upgrade.
