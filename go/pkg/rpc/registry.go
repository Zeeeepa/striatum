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

var methodEntries = []MethodEntry{
	NewMethod("daemon.hello", nil, false, ""),
	NewMethod("daemon.describe", CapPtr(CapabilityRead), false, ""),
	NewMethod("status", CapPtr(CapabilityRead), true, ""),
	NewMethod("why", CapPtr(CapabilityRead), true, ""),
	NewMethod("doctor", CapPtr(CapabilityRead), true, ""),
	NewMethod("dashboard", CapPtr(CapabilityRead), true, ""),
	NewMethod("dashboard.all", CapPtr(CapabilityRead), false, ""),
	NewMethod("evidence.export", CapPtr(CapabilityRead), true, ""),
	NewMethod("workflow.validate", CapPtr(CapabilityWrite), true, ""),
	NewMethod("workflow.generate.preview", CapPtr(CapabilityRead), true, ""),
	NewMethod("workflow.generate", CapPtr(CapabilityWrite), true, ""),
	NewMethod("run.prepare", CapPtr(CapabilityWrite), true, ""),
	NewMethod("run.start", CapPtr(CapabilityWrite), true, ""),
	NewMethod("session.register", CapPtr(CapabilityWrite), true, ""),
	NewMethod("ack", CapPtr(CapabilityWrite), true, ""),
	NewMethod("block", CapPtr(CapabilityWrite), true, ""),
	NewMethod("heartbeat", CapPtr(CapabilityWrite), true, ""),
	NewMethod("publish_artifact", CapPtr(CapabilityWrite), true, ""),
	NewMethod("complete", CapPtr(CapabilityWrite), true, ""),
	NewMethod("release", CapPtr(CapabilityWrite), true, ""),
	NewMethod("recovery.stale_leases", CapPtr(CapabilityRecovery), true, ""),
	NewMethod("recovery.requeue_stale", CapPtr(CapabilityRecovery), true, ""),
	NewMethod("recovery.cancel_job", CapPtr(CapabilityRecovery), true, ""),
	NewMethod("recovery.process_reconcile", CapPtr(CapabilityRecovery), true, ""),
	NewMethod("recovery.resume", CapPtr(CapabilityRecovery), true, ""),
	NewMethod("claim_next", CapPtr(CapabilityClaim), true, ""),
	NewMethod("verdict", CapPtr(CapabilityReview), true, ""),
	NewMethod("submit_review", CapPtr(CapabilityReview), true, ""),
	NewMethod("dogfood.publish_on_behalf", CapPtr(CapabilityWrite), true, ""),
	NewMethod("dogfood.surgical_recovery", CapPtr(CapabilitySurgicalRecovery), true, ""),
	NewMethod("supervise.start", CapPtr(CapabilityWrite), true, ""),
	NewMethod("supervise.send", CapPtr(CapabilityWrite), true, ""),
	NewMethod("supervise.stop", CapPtr(CapabilityWrite), true, ""),
	NewMethod("supervise.status", CapPtr(CapabilityRead), true, ""),
	NewMethod("supervise.list", CapPtr(CapabilityRead), true, ""),
	NewMethod("supervise.reattach_status", CapPtr(CapabilityRead), true, ""),
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
