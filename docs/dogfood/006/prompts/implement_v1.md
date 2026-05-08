# Implement V1 Slice

Verify a human acceptance decision exists under
docs/dogfood/006/decisions/. If absent, block.

Ship the V1 build slice exactly as the synthesis specifies. Add
src/striatum/service.py, wire striatum serve to it via parser/dispatch,
add tests covering every acceptance criterion. Run lint/typecheck/test
before publishing BUILD_HANDOFF.md.
