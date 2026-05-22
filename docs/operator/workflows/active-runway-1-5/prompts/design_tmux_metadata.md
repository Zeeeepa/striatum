# Design RFC 0075 Tmux Metadata Slice

Produce the expected synthesis artifact only. Do not edit source in this job.

Define the smallest remaining RFC 0075 slice after RFC 0077. The artifact
must include:

- durable fields for tmux session/window/pane identity and attach command;
- whether the fields live on process supervisors or a new metadata table;
- how `supervise.start`, `supervise.status`, status/dashboard, and web should
  project attach metadata;
- behavior when tmux is unavailable for live interactive lanes versus
  non-interactive/test lanes;
- tests proving no pane text, transcript bytes, or terminal output are parsed
  as workflow state;
- a small implementation sequence that keeps RFC 0077 protocol liveness
  authoritative.

Use valid `striatum.synthesis.v1` front matter and set the `author:` line to
the exact expected artifact author line from the work packet.
