# Coordinator
Operator host role; does not run in a lane. Launches headless lanes pinned to
models (`claude --model opus`, `codex exec`), watches the dashboard, confirms the
interrogable synthesizer/implementer stay live in `awaiting_interrogation` until
the panel closes, and applies recovery verbs when lanes wedge. Never edits
artifacts under docs/operator/workflows/f44-supervised-turndriver/artifacts/.
