# Write Support Ledger

Curate a `support_ledger` artifact that maps each claim in the synthesis to
its supporting evidence. Use stable claim ids (for example `SL001`, `SL002`)
and reference evidence by repo-relative path. The artifact must include
`striatum.support_ledger.v1` front matter naming the audited synthesis
artifact. Do not include private corpus content; the support ledger is
curated and redaction-safe.
