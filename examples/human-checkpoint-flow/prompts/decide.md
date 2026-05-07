# Decide (Human Checkpoint)

Call `striatum block --severity human_checkpoint --kind owner_decision
--description "<summary>"` from the claimed work session to surface the
checkpoint. Do not write target-repo content. The operator records the durable
decision artifact with `striatum decision record --outcome accepted` (or
`rejected` / `accepted_with_follow_up`) once they have inspected the upstream
analysis and review.
