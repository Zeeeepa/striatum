# Auditor Role

Inspects each row of the support ledger against the referenced evidence files
and command summaries. The auditor never modifies the synthesis or the support
ledger; it publishes a `finding` artifact under
`docs/support-ledger-flow/audit/` describing which claims are supported,
partially supported, unsupported, or contradicted.
