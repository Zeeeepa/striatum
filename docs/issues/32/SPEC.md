# GH #32 - skills install supervised wrapper copy

Source: https://github.com/halbritt/striatum/issues/32

## Summary

Downstream repositories that run `striatum skills install` can miss
`.striatum/bin/codex-supervised-wrapper.sh`, causing supervised codex lanes to
fail with `No such file or directory`.

## Acceptance

1. `striatum skills install` ensures canonical supervised wrappers are present
   in the target repository operational scratch.
2. The wrapper copy is idempotent and keeps executable bits.
3. Tests cover the downstream-consumer path.
