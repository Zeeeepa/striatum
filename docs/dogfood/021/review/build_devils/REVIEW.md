# Build review (devils_advocate): RFC 0023 V1

author: reviewer-claude-opus-004
date: 2026-05-09
verdict: accept

Devil's-advocate posture on the V1 build.

## Verdict

**accept** — the implementation matches the synthesis and the V1.5 deferrals are honest.

## Counterarguments

### "Did scope expand silently?"

The synthesis pinned chat + view + artifact-Markdown. The build delivered exactly those three; full file-tree UI is genuinely deferred (no `/browse/<path>` directory listing endpoint shipped). No scope creep. **Accept.**

### "Are V1.5 deferrals real?"

Yes. The handoff names the V1.5 candidates concretely (tool use, supervised-CLI backend, file-tree UI, etc.). None of them have skeleton code that would lock V1 into V1.5's shape. **Accept.**

### "Does the implementation match the synthesis?"

- Two flavors implemented (`anthropic_messages` + `openai_chat`)? Yes; tested with mock servers.
- URL scheme validation? Yes; tested.
- CSP unchanged? Yes; tested.
- Mutation gating on `POST /chat/new` and `/send`? Yes (returns 405 without `--allow-mutations`).
- Transcripts in scratch only? Yes; no SQLite writes; gitignored.
- Markdown rendering on artifact pages? Yes; the dispatch reads `.md` artifacts via `relative_to(repo_root)` for safety.
- Empty-state on chat-index when unconfigured? Yes; the page shows the four env-var lines as a copy-pasteable `<pre>`.

### "Anything inconsistent with the synthesis?"

The synthesis recommended an OS-keyring deferral; the implementation doesn't add it (which is correct). No inconsistencies found.

## Decision

Accept. Build matches synthesis; V1.5 deferrals are honest; no scope expansion.
