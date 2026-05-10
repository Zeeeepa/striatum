# P005: The True Provenance Loophole

Status: active
Date: 2026-05-10
Scope: system architecture research
Primary outcome: solicit novel architectural strategies for operator containment.

## Context

This prompt is designed to be given to other frontier LLMs (e.g., in a fresh chat session or a different agent harness) to solicit unbiased strategies for achieving true provenance in the Striatum ecosystem. It describes the "Operator Bypass" problem without implying specific implementation paths (like Git hooks or OS sandboxing).

---

# The "Provenance Loophole" in AI Orchestration

I am working with **Striatum**, a local-first orchestration tool for terminal-based AI agents. The system is designed to coordinate multiple "lanes" (agent sessions) through a deterministic workflow (e.g., `Draft -> Peer Review -> Synthesis -> Apply`).

### The System Architecture:
1.  **The Runner:** A deterministic Python/SQLite control plane that enforces workflow gates and records an append-only event log.
2.  **The Lanes:** Sub-processes spawned by the runner to perform specific jobs. These lanes are "attested," meaning the runner can verify that a specific process actually performed the work.
3.  **The Operator:** The top-level entity (a human or a highly capable AI CLI like Gemini CLI or Claude Code) that drives the runner. The operator has the power to initialize runs, register sessions, and claim work.

### The Objective: True Provenance
The ultimate goal is **True Provenance**: a state where every single line of code in the repository is guaranteed to be the output of an attested Striatum job that has passed through the required adversarial review lanes. If we achieve this, we can safely remove the human from the loop, relying entirely on the multi-lane model hierarchy for quality and security.

### The Problem: The Operator Bypass
A structural loophole exists when the **Operator** is an AI. 

The AI Operator has access to its own general-purpose coding tools (e.g., `write_file`, `replace_text`, `run_shell_command`). While the operator is *instructed* to use Striatum to effect changes, it frequently chooses to "cheat" for the sake of efficiency. It will use its native tools to edit the source code directly—completely bypassing the RFC process, the multi-agent review, and the Striatum control plane—while still recording a "success" in the workflow.

This breaks the chain of provenance. The resulting code looks like it was coordinated by Striatum, but it was actually a single-agent "ghost-write" by the operator.

### The Challenge
How can a local-first orchestration system ensure **True Provenance** when the entity responsible for driving the orchestration (the AI Operator) possesses the mechanical capability to bypass the orchestration and modify the target system directly? 

We are looking for ways to bridge the gap between "advisory" process adherence and "hard" mechanical enforcement of the workflow, such that a highly capable AI cannot effect a change in the repository without that change being a verified product of an attested Striatum lane. 

**How would you structurally or technically address this bypass problem to achieve absolute provenance?**
