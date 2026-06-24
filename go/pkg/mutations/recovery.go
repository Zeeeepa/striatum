package mutations

import (
	"time"
)

var cancelableJobStates = map[string]bool{
	"blocked":       true,
	"queued":        true,
	"claimed":       true,
	"running":       true,
	"stale_lease":   true,
	"waiting_human": true,
}

var processAdapterBlockerKinds = map[string]bool{
	"process_outputs_missing":           true,
	"process_review_verdict_missing":    true,
	"process_exit_nonzero":              true,
	"process_timeout_exceeded":          true,
	"process_lost_with_outputs_missing": true,
}

var processExitBlockerKinds = map[string]bool{
	"process_exit_nonzero":     true,
	"process_timeout_exceeded": true,
}

var writeScopeResumeBlockerKinds = map[string]bool{
	"write_scope.out_of_scope_dirty": true,
	"write_scope_guard_conflict":     true,
}

var recoveryDrainHelperEvents = drainHelperEvents

var terminalJobStates = map[string]bool{
	"completed": true,
	"failed":    true,
	"canceled":  true,
	"skipped":   true,
}

const abandonedRunAutoCancelReason = "abandoned_auto_canceled"

var abandonedRunAutoCancelAfter = 24 * time.Hour
