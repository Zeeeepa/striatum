---
schema_version: "striatum.findings_ledger.v1"
artifact_kind: "findings_ledger"
summary_count: 5
---

# FINDINGS LEDGER — F42 conversation turn-driver design review

author: cross_examiner-gemini_3.5_flash-1

This ledger records the findings of the design review for the F42 conversation turn-driver. The review evaluated the proposed launch surface, wait primitives, credential scrubbing boundaries, lease management, and error handling for robustness, correctness, and compatibility with the Striatum daemon protocols.

## Findings

### Finding 1: Conversation turns do not carry a job lease, violating supervisor heartbeat assumptions
*   **Severity:** Medium
*   **Description:** The design synthesis (§5) and Claude Code design (§2.5) suggest that the turn driver should heartbeat the work lease of the supervised lane while waiting for or generating a turn. However, conversation messages delivered via `deliverPendingConversationTurn` (and returned through `work.await_packet`) are symmetric, floor-derived queue messages that do *not* contain a job lease (`lease_id` is absent).
*   **Impact:** If the driver attempts to invoke `work.heartbeat` using a missing or dummy lease ID, the daemon will reject the call. If the driver does not heartbeat, the supervisor might falsely classify the lane as stalled if its liveness detection is tied to lease activity rather than general daemon protocol activity.
*   **Recommendation:** Clarify that conversation lanes are lease-free at the job level. Ensure the supervisor's liveness classifier utilizes `sessionliveness` records updated via `tools/list` and `work.await_packet` rather than requiring `work.heartbeat` calls.

### Finding 2: Free variable `convID` in wait loop pseudocode leads to resolution failure
*   **Severity:** Low
*   **Description:** In the wait loop pseudocode (§3 of `DESIGN_SYNTHESIS.md`), when `AwaitTurn` returns `NoWork` or `NotOurFloor`, the driver calls `conv.Show(ctx, convID).Closed`. At this point, the conversation ID (`convID`) is a free variable. If the session participates in multiple conversations, or if it is newly registered and hasn't yet received a turn message, the `convID` is unknown to the loop.
*   **Impact:** The driver will fail to resolve the conversation ID to query, preventing it from detecting conversation closure when it does not hold the floor.
*   **Recommendation:** Initialize the turn-driver with the target `conversation_id` explicitly (e.g. from the supervisor configuration), or require the driver to list active conversations via `conversation.list` to track closed states.

### Finding 3: Blocking `work.await_packet` call prevents concurrent health checks and closure checks
*   **Severity:** Medium
*   **Description:** The wait loop uses `work.await_packet` which blocks inside `HandleAwaitPacket` for up to 30 seconds. While the driver is blocked on this synchronous HTTP POST request, it cannot concurrently query `conversation.show` to check if another participant has closed the conversation.
*   **Impact:** If a conversation is closed while the driver is waiting, the driver will remain blocked for the duration of the 30-second poll before it can observe the closure. This introduces a 30s exit delay and potential lease/liveness tracking stalls.
*   **Recommendation:** Decouple the await and closure checks. Run the loop in a non-blocking concurrent manner (e.g., using Go channels/goroutines), or update the daemon to instantly return/cancel `work.await_packet` when the associated conversation changes state to closed.

### Finding 4: Security limit of environment-based credential scrubbing
*   **Severity:** Low
*   **Description:** Section 4 of the design synthesis implements environment scrubbing (removing `STRIATUM_MCP_URL`, `STRIATUM_MCP_TOKEN`, etc.) to prevent the single-shot model from accessing the daemon. However, since the driver and the model run under the same OS user, a malicious model could read the capability token from the disk cache (e.g. `.striatum/` client token files) or connect to default daemon socket paths, bypassing environment boundaries.
*   **Impact:** The credential scrub provides a strong boundary for well-behaved/accidental leakage but does not provide complete process privilege isolation against a compromised agent.
*   **Recommendation:** Document that environment scrubbing is a credential leakage guard, not a complete security boundary. For true process isolation, the supervised runner should eventually support running the subprocess under a restricted OS user or namespace.

### Finding 5: Content sanitization ignores carriage returns used for line overwriting
*   **Severity:** Low
*   **Description:** The sanitizer is designed to strip ANSI escape sequences and control bytes except `\n` and `\t`. Some model CLIs output carriage returns (`\r`) as part of progress logging or terminal formatting.
*   **Impact:** Blindly stripping `\r` or converting it without line-overwrite handling could result in garbled text (overwritten lines concatenated together) in the final conversation transcript.
*   **Recommendation:** Explicitly define carriage return (`\r`) handling in the sanitizer (e.g., by splitting lines or discarding prefix lines up to `\r` if they emulate overwriting) rather than stripping them as general control characters.
