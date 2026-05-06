# agent_runner Specification

Status: pending PRD
Date: 2026-05-06

The technical specification will be written after the PRD and early design
decisions establish the product boundary.

Seed direction:

- local coordinator state store for live workflow state and messages;
- repository artifacts for durable findings, syntheses, markers, and commits;
- named agent lanes for exact terminal model commands;
- tmux as a PTY adapter, not the source of truth;
- explicit review verdicts, revision lanes, and hard stop conditions.
