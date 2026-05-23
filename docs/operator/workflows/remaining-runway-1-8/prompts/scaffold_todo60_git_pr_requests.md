# Scaffold TODO 60 Git/PR Request Artifacts

Produce the expected scaffold artifact only. Do not edit source, tests, TODO,
roadmap, or the operator brief in this job.

Focus on the D127 follow-up after read-only `git.snapshot`: durable
commit-request and PR-request artifacts plus explicit local commit
confirmation. Hosted provider actions remain out of core.

The scaffold must include:

- artifact contracts for commit requests and PR requests;
- local commit-confirmation flow, confirmation evidence, and refusal cases;
- how read-only snapshot evidence feeds request artifacts;
- UI/MCP/CLI surface notes without adding hosted-provider behavior;
- implementation write scopes, tests, and audit/receipt expectations;
- explicit non-scope for push, fetch, hosted PR creation, provider SDKs,
  credentials, remote URLs, or external persistence in core.

Use valid `striatum.synthesis.v1` front matter and set the `author:` line to
the exact expected artifact author line from the work packet.
