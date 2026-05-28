# GH #35 - zombie supervised lane recovery

Source: https://github.com/halbritt/striatum/issues/35

## Summary

When a supervised lane process becomes a zombie, the daemon can keep reporting
it as attached and `supervise.stop` can hang behind transaction locks.

## Acceptance

1. Zombie or already-dead supervised processes are classified as lost or
   stopped without blocking on SIGTERM waits.
2. `supervise stop` against a zombie succeeds.
3. PostgreSQL transactions used by supervision control are closed on all paths.
