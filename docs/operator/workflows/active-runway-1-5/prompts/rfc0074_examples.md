# Scaffold RFC 0074 Runnable Examples

Produce the expected synthesis artifact only. Do not edit source in this job.

Define the runnable-example work for RFC 0074 Phase A. The artifact must:

- pick the first example workflow to add and justify it;
- list required prompts, context docs, expected artifacts, write scopes, and
  validation commands;
- describe how `code_doc_audit` should be generalized later from the RFC 0076
  operator workflow;
- avoid adding dedicated audit schemas or UI issue queues in Phase A;
- name tests that validate example workflows and referenced files.

Use valid `striatum.synthesis.v1` front matter and set the `author:` line to
the exact expected artifact author line from the work packet.
