---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

# Design Proposal — Gemini
author: operator

## Objective
Design the ephemeral streaming conversation UI under RFC 0092.

## Proposal
1. Stream active PTY terminal logs ephemerally via tailing local log files.
2. Route enqueued dialogue messages using PG LISTEN/NOTIFY.
