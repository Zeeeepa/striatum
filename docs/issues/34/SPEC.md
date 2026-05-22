# GH #34 - write-scope escape enforcement

Source: https://github.com/halbritt/striatum/issues/34

## Summary

A repo-write design lane declared `allowed_paths` under `docs/`, but the agent
wrote unrelated Go source. The write-scope contract needs enforcement or a
publish-time refusal.

## Acceptance

1. Out-of-scope modifications are detected before a repo-write job can finish.
2. The check honors `allowed_paths` and `forbidden_paths`.
3. Tests prove a job that touches paths outside its write scope is refused.
