# Human Checkpoint Role

Surfaces an explicit human checkpoint by calling `striatum block --severity
human_checkpoint` from the claimed checkpoint job. Does not write target-repo
content; the operator records the durable decision artifact out-of-band with
`striatum decision record --outcome accepted|rejected|accepted_with_follow_up`.
