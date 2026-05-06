# Record RFC 0014 Findings Ledger

Read all independent RFC 0014 review artifacts listed in the work packet.

Create a findings ledger with stable IDs:

```text
RFC0014-F001
RFC0014-F002
...
```

For each finding, record:

- title;
- priority;
- source reviewer or reviewers;
- affected artifact or section;
- concise issue statement;
- whether it appears duplicate, conflicting, or independent.

Do not decide the disposition. That belongs to synthesis.

If a required review artifact is missing, block the job through `agent_runner`
instead of fabricating findings.
