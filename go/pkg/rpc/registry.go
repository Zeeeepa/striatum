package rpc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

type Capability string

const (
	CapabilityRead             Capability = "read"
	CapabilityWrite            Capability = "write"
	CapabilityReview           Capability = "review"
	CapabilityClaim            Capability = "claim"
	CapabilityApply            Capability = "apply"
	CapabilityAdmin            Capability = "admin"
	CapabilityRecovery         Capability = "recovery"
	CapabilitySurgicalRecovery Capability = "surgical_recovery"
)

var Capabilities = map[Capability]struct{}{
	CapabilityRead:             {},
	CapabilityWrite:            {},
	CapabilityReview:           {},
	CapabilityClaim:            {},
	CapabilityApply:            {},
	CapabilityAdmin:            {},
	CapabilityRecovery:         {},
	CapabilitySurgicalRecovery: {},
}

type ScopeMode string

const (
	ScopeSingleRepo   ScopeMode = "single_repo"
	ScopeCrossRepo    ScopeMode = "cross_repo"
	ScopeDaemonGlobal ScopeMode = "daemon_global"
)

type MethodEntry struct {
	Method              string      `json:"method"`
	RequiredCapability  *Capability `json:"required_capability"`
	RepositoryScope     bool        `json:"repository_scope"`
	RepositoryScopeMode ScopeMode   `json:"repository_scope_mode"`
	ParamsSchemaVersion int         `json:"params_schema_version"`
	AuditClass          string      `json:"audit_class"`
	MinEnvelope         int         `json:"min_envelope"`
	Deprecated          bool        `json:"deprecated"`
}

func NewMethod(method string, capability *Capability, repositoryScope bool, mode ScopeMode) MethodEntry {
	if mode == "" {
		if repositoryScope {
			mode = ScopeSingleRepo
		} else {
			mode = ScopeDaemonGlobal
		}
	}
	return MethodEntry{
		Method:              method,
		RequiredCapability:  capability,
		RepositoryScope:     repositoryScope,
		RepositoryScopeMode: mode,
		ParamsSchemaVersion: 1,
		AuditClass:          "metadata",
		MinEnvelope:         1,
	}
}

func CapPtr(capability Capability) *Capability {
	return &capability
}

func DeprecatedAlias(method string, capability Capability) MethodEntry {
	entry := NewMethod(method, CapPtr(capability), true, "")
	entry.Deprecated = true
	return entry
}

var methodEntries = []MethodEntry{
	NewMethod("daemon.hello", nil, false, ""),
	NewMethod("daemon.describe", CapPtr(CapabilityRead), false, ""),
	NewMethod("status", CapPtr(CapabilityRead), true, ""),
	NewMethod("why", CapPtr(CapabilityRead), true, ""),
	NewMethod("doctor", CapPtr(CapabilityRead), true, ""),
	NewMethod("dashboard", CapPtr(CapabilityRead), true, ""),
	NewMethod("dashboard.all", CapPtr(CapabilityRead), false, ""),
	NewMethod("evidence.export", CapPtr(CapabilityRead), true, ""),
	NewMethod("corpus.export", CapPtr(CapabilityRead), true, ""),
	NewMethod("run.summary", CapPtr(CapabilityRead), true, ""),
	NewMethod("run.graph", CapPtr(CapabilityRead), true, ""),
	NewMethod("workflow.validate", CapPtr(CapabilityRead), true, ""),
	NewMethod("workflow.plan", CapPtr(CapabilityRead), true, ""),
	NewMethod("workflow.graph", CapPtr(CapabilityRead), true, ""),
	NewMethod("workflow.templates.list", CapPtr(CapabilityRead), true, ""),
	NewMethod("workflow.templates.show", CapPtr(CapabilityRead), true, ""),
	NewMethod("workflow.generate.preview", CapPtr(CapabilityRead), true, ""),
	NewMethod("list.runs", CapPtr(CapabilityRead), true, ""),
	NewMethod("list.sessions", CapPtr(CapabilityRead), true, ""),
	NewMethod("list.jobs", CapPtr(CapabilityRead), true, ""),
	NewMethod("list.artifacts", CapPtr(CapabilityRead), true, ""),
	NewMethod("list.workflows", CapPtr(CapabilityRead), true, ""),
	NewMethod("worktree.list", CapPtr(CapabilityRead), true, ""),
	NewMethod("repo.list", CapPtr(CapabilityRead), false, ""),

	NewMethod("session.register", CapPtr(CapabilityClaim), true, ""),
	NewMethod("session.close", CapPtr(CapabilityClaim), true, ""),
	NewMethod("work.claim_next", CapPtr(CapabilityClaim), true, ""),
	NewMethod("work.ack", CapPtr(CapabilityClaim), true, ""),
	NewMethod("work.heartbeat", CapPtr(CapabilityClaim), true, ""),
	NewMethod("work.release", CapPtr(CapabilityClaim), true, ""),
	NewMethod("supervise.start", CapPtr(CapabilityClaim), true, ""),
	NewMethod("supervise.send", CapPtr(CapabilityClaim), true, ""),
	NewMethod("supervise.stop", CapPtr(CapabilityClaim), true, ""),
	NewMethod("supervise.status", CapPtr(CapabilityRead), true, ""),
	NewMethod("supervise.list", CapPtr(CapabilityRead), true, ""),
	NewMethod("supervise.reattach_status", CapPtr(CapabilityRead), true, ""),

	NewMethod("work.send_message", CapPtr(CapabilityWrite), true, ""),
	NewMethod("work.block", CapPtr(CapabilityWrite), true, ""),
	NewMethod("work.complete", CapPtr(CapabilityWrite), true, ""),
	NewMethod("artifact.publish", CapPtr(CapabilityWrite), true, ""),
	NewMethod("worktree.create", CapPtr(CapabilityWrite), true, ""),
	NewMethod("worktree.release", CapPtr(CapabilityWrite), true, ""),
	NewMethod("workflow.init", CapPtr(CapabilityWrite), true, ""),
	NewMethod("workflow.generate", CapPtr(CapabilityWrite), true, ""),
	NewMethod("workflow.upgrade", CapPtr(CapabilityWrite), true, ""),
	NewMethod("dogfood.publish_on_behalf", CapPtr(CapabilityWrite), true, ""),

	NewMethod("review.submit", CapPtr(CapabilityReview), true, ""),
	NewMethod("review.verdict", CapPtr(CapabilityReview), true, ""),

	NewMethod("review.override", CapPtr(CapabilityAdmin), true, ""),
	NewMethod("decision.record", CapPtr(CapabilityAdmin), true, ""),
	NewMethod("checkpoint.resolve", CapPtr(CapabilityAdmin), true, ""),
	NewMethod("branch.confirm", CapPtr(CapabilityAdmin), true, ""),
	NewMethod("run.prepare", CapPtr(CapabilityAdmin), true, ""),
	NewMethod("run.start", CapPtr(CapabilityAdmin), true, ""),
	NewMethod("run.pause", CapPtr(CapabilityAdmin), true, ""),
	NewMethod("run.resume", CapPtr(CapabilityAdmin), true, ""),
	NewMethod("run.cancel", CapPtr(CapabilityAdmin), true, ""),
	NewMethod("run.retry_job", CapPtr(CapabilityAdmin), true, ""),
	NewMethod("repo.init", CapPtr(CapabilityAdmin), true, ""),

	NewMethod("recovery.stale_leases", CapPtr(CapabilityRecovery), true, ""),
	NewMethod("recovery.requeue_stale", CapPtr(CapabilityRecovery), true, ""),
	NewMethod("recovery.cancel_job", CapPtr(CapabilityRecovery), true, ""),
	NewMethod("recovery.process_reconcile", CapPtr(CapabilityRecovery), true, ""),
	NewMethod("recovery.resume", CapPtr(CapabilityRecovery), true, ""),
	NewMethod("recovery.auto", CapPtr(CapabilityRecovery), true, ""),
	NewMethod("recovery.watch", CapPtr(CapabilityRecovery), true, ""),

	NewMethod("dogfood.surgical_recovery", CapPtr(CapabilitySurgicalRecovery), true, ""),

	NewMethod("apply.reviewed_patch", CapPtr(CapabilityApply), true, ""),
	NewMethod("apply.receipt.show", CapPtr(CapabilityRead), true, ""),
	NewMethod("apply.receipt.verify", CapPtr(CapabilityRead), true, ""),
	NewMethod("repo.add", CapPtr(CapabilityAdmin), false, ""),
	NewMethod("repo.remove", CapPtr(CapabilityAdmin), false, ""),
	NewMethod("daemon.token.create", CapPtr(CapabilityAdmin), false, ""),
	NewMethod("daemon.token.revoke", CapPtr(CapabilityAdmin), false, ""),
	NewMethod("daemon.token.rotate", CapPtr(CapabilityAdmin), false, ""),
	NewMethod("cross_repo.list", CapPtr(CapabilityRead), false, ScopeCrossRepo),
	NewMethod("cross_repo.describe", CapPtr(CapabilityRead), false, ScopeCrossRepo),
	NewMethod("cross_repo.why", CapPtr(CapabilityRead), false, ScopeCrossRepo),
	NewMethod("cross_repo.cancel", CapPtr(CapabilityRecovery), false, ScopeCrossRepo),
	NewMethod("daemon.key.rotate", CapPtr(CapabilityAdmin), false, ""),
	NewMethod("daemon.shutdown", CapPtr(CapabilityAdmin), false, ""),
	NewMethod("daemon.migrate", CapPtr(CapabilityAdmin), false, ""),
	NewMethod("daemon.migrate_repo_local", CapPtr(CapabilityAdmin), false, ""),

	DeprecatedAlias("ack", CapabilityClaim),
	DeprecatedAlias("heartbeat", CapabilityClaim),
	DeprecatedAlias("release", CapabilityClaim),
	DeprecatedAlias("block", CapabilityWrite),
	DeprecatedAlias("complete", CapabilityWrite),
	DeprecatedAlias("publish_artifact", CapabilityWrite),
	DeprecatedAlias("claim_next", CapabilityClaim),
	DeprecatedAlias("verdict", CapabilityReview),
	DeprecatedAlias("submit_review", CapabilityReview),
}

var MethodRegistry = buildRegistry(methodEntries)

func buildRegistry(entries []MethodEntry) map[string]MethodEntry {
	registry := make(map[string]MethodEntry, len(entries))
	for _, entry := range entries {
		registry[entry.Method] = entry
	}
	return registry
}

func DescribeMethods() map[string]any {
	entries := SortedMethods()
	return map[string]any{
		"methods_etag": MethodsETag(),
		"methods":      entries,
	}
}

func SortedMethods() []MethodEntry {
	entries := append([]MethodEntry(nil), methodEntries...)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Method < entries[j].Method
	})
	return entries
}

func MethodsETag() string {
	payload, _ := json.Marshal(SortedMethods())
	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:])
}
