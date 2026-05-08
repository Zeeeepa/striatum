# Role: coordinator (dogfood 002)

The coordinator role is the configured chat partner for this run.
For dogfood 002, the human operator is the coordinator — there is no
separate coordinator job. This role exists in the workflow so the
deterministic runner has a default lane to attribute non-job
operations to (e.g., the human-recorded decisions during checkpoints).
