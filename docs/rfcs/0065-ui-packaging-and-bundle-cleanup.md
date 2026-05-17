# RFC 0065: UI Packaging and Bundle Cleanup

## Status
Implemented

## Summary
The web UI packaging cleanup landed: bundle output is reproducible, generated
assets are cleaned before rebuild, wheel/package size guardrails exist, and
dependency cleanup is tracked through CI. Manual chunking remains monitor-only
unless bundle growth makes it necessary.

## Motivation
The React/Vite UI added a contributor-side toolchain while operator installs
remain pip-only. Packaging had to prove that generated output is deterministic
and that bundle growth cannot silently bloat the Python wheel.

## Proposed Implementation
Implemented pieces include `ui-clean`, bundle hash/size checks, wheel-content
coverage, dependency cleanup, and docs that frame manual chunking as an
observability threshold rather than a premature architecture requirement.
