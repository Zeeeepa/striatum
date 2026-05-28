# GH #33 - concurrent supervise start deadlock

Source: https://github.com/halbritt/striatum/issues/33

## Summary

Concurrent `striatum supervise start` RPCs can deadlock in PostgreSQL and leave
daemon connections idle in transaction or waiting on updates.

## Acceptance

1. Supervise start uses deterministic transaction/lock ordering or explicit
   serialization so concurrent starts do not deadlock.
2. Transactions are always closed on error.
3. Tests cover the guardrail or lock acquisition path.
