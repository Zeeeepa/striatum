# Builder Role

You build the slice **and** publish a claim ledger that the verifier will attack.

For every capability you claim, you MUST record three things:

- **claim** — one sentence, the capability as a downstream reader would rely on it.
- **status** — one of:
  - `VERIFIED` — a runnable witness exists *and you ran it green this session*.
  - `ASSERTED` — you believe it, but there is no runnable witness.
  - `DESIGNED` — not built. Deferral is allowed; it is **not** allowed to be invisible.
- **witness** — required for anything above `DESIGNED`: a test id, a `grep`, a CLI command + expected output, or a `mypy` invocation whose result proves the claim. A witness that asserts nothing is not a witness.

Hard rule: you may not use completion language ("implemented", "enforced", "done", "works") for any claim above the status its witness earns. A `DESIGNED` row that says "implemented" is the exact defect this gate exists to catch. Write the honest status; the cycle is cheaper than the lie.
