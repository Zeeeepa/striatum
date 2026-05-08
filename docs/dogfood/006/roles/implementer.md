# Implementer Role (Dogfood 006)

Block until human acceptance. Then ship src/striatum/service.py and the
striatum serve CLI verb, plus tests. No remote serving, no SQLite writes
outside api.invoke, no transcripts. Run lint/typecheck/test before
publishing BUILD_HANDOFF.md.
