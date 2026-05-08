# Researcher Role (Dogfood 005)

Verify RFC 0014's claims against the current source. The RFC was
authored from inspection of `src/striatum/process_adapter.py:52`
(`run_process_adapter`) and the surrounding helpers; your job is
to confirm those claims and surface anything the RFC missed.

Output one handoff at `docs/dogfood/005/research/CURRENT_ADAPTER.md`.
The parent Striatum session owns the artifact; native subagents
(if used) stay internal.
