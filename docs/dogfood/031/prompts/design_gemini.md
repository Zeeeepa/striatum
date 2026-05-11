# Gemini Design Prompt

Produce `docs/dogfood/031/design/gemini/DESIGN.md`.

Design an implementation plan for the RFC 0028 V1 acceptance-criteria slice with emphasis on operational reality across macOS, Linux, and Windows. Treat platform differences as first-class risks, not footnotes.

Your plan must cover:

- daemon process management on each platform: launchd plist on macOS, systemd user unit on Linux, Windows service or scheduled task on Windows, including install/uninstall flows and where logs are written;
- transport availability per platform: Unix-domain socket on macOS/Linux, named pipe or loopback TCP on Windows, with explicit owner-only permission handling on each;
- single-binary distribution options if a daemon ships separately from the CLI client, including how this interacts with the current Python packaging (`pyproject.toml`, `.venv`);
- pidfile, socket path, and log directory conventions per platform and how they map to XDG-style or platform-native defaults;
- upgrade and version skew handling: how a CLI built against daemon vN behaves against daemon vN+1, and how the daemon refuses unsafe downgrades;
- crash recovery: what state the daemon must persist before exit to safely resume supervised processes, scheduled recovery sweeps, and active client capabilities after restart;
- daemon log handling that does not become transcript-like sensitive material: rotation, redaction of capability tokens, refusal to capture supervised-agent stdout/stderr beyond structured events;
- digest/identity of registered repositories: how the daemon refuses symlink and path-traversal tricks at `striatum repo add`, and how repository renames or moves are handled safely;
- migration of existing `.striatum/state.sqlite3` runs across all three platforms, including filesystem case-sensitivity and path-length pitfalls;
- adversarial test plan with concrete cases for: hostile local client on the same UID, prompt-injected MCP client requesting mutation, registry tamper between daemon restarts, repository unregister while a supervised agent is mid-job, downgrade-attack upgrade paths, and stale supervised processes after daemon crash;
- staged rollout that ships the V1 slice on at least Linux first, with explicit gates for macOS and Windows parity.

State which parts are implementable in the current local CLI architecture and which require platform-specific work or are deferred. If the work packet supplies an `author:` line, copy it exactly into the artifact title block.
