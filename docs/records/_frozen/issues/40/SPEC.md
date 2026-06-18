# GH #40 - RFC 0075 tmux attach metadata surfaces

Source: https://github.com/halbritt/striatum/issues/40

## Summary

RFC 0075 requires daemon-owned tmux session/window/pane metadata, a local attach
command, and status/dashboard/current-brief projections without making pane
contents authoritative.

## Acceptance

1. Supervisor metadata includes tmux identifiers and a safe attach command.
2. Status, dashboard/current-brief, and relevant read APIs project the metadata.
3. Tests cover metadata presence, absence, and stale/unattached classification.
