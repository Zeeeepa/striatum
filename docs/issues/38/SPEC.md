# GH #38 - RFC 0075 MCP liveness timestamps and classifications

Source: https://github.com/halbritt/striatum/issues/38

## Summary

RFC 0075 needs daemon-owned MCP activity timestamps and deadline
classifications for discovery, await, ack, heartbeat, question, and escalation
stalls. Terminal output and transcripts must remain non-authoritative.

## Acceptance

1. The daemon persists protocol activity timestamps.
2. A daemon-owned classifier emits metadata-only missed/recovered liveness
   events.
3. Status/read surfaces expose classifications without storing transcripts.
4. Focused fake-agent tests cover the requested stall classes.
