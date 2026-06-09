package mcp

import (
	"context"
	"sort"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

type Tool struct {
	Name                string         `json:"name"`
	Description         string         `json:"description"`
	InputSchema         map[string]any `json:"inputSchema"`
	RequiredCapability  string         `json:"required_capability"`
	RepositoryScopeMode string         `json:"repository_scope_mode"`
}

const discoveryRepositoryID = "__striatum_mcp_tools_list_discovery__"

func VisibleTools(ctx context.Context, authorizer rpc.Authorizer, token string, repositoryID string) []Tool {
	tools := []Tool{}
	for _, entry := range visibleMethodEntries() {
		if entry.RequiredCapability == nil || isInternal(entry.Method) || isHiddenProductionTool(entry.Method) || entry.Deprecated {
			continue
		}
		scopeRepo := ""
		if entry.RepositoryScopeMode == rpc.ScopeSingleRepo {
			scopeRepo = repositoryID
			if scopeRepo == "" {
				scopeRepo = discoveryRepositoryID
			}
		}
		auth := authorizer.Authorize(entry.RequiredCapability, scopeRepo, token)
		if auth.Decision != "allowed" && !isRepositoryDiscoveryAllowed(entry, repositoryID, auth) {
			continue
		}
		tools = append(tools, Tool{
			Name:                entry.Method,
			Description:         "Striatum daemon RPC method " + entry.Method,
			InputSchema:         inputSchema(entry),
			RequiredCapability:  string(*entry.RequiredCapability),
			RepositoryScopeMode: string(entry.RepositoryScopeMode),
		})
	}
	return tools
}

func isRepositoryDiscoveryAllowed(entry rpc.MethodEntry, requestedRepositoryID string, auth rpc.AuthContext) bool {
	return requestedRepositoryID == "" &&
		entry.RepositoryScopeMode == rpc.ScopeSingleRepo &&
		auth.DenialReason == "capability_scope_mismatch"
}

func visibleMethodEntries() []rpc.MethodEntry {
	entries := rpc.SortedMethods()
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		seen[entry.Method] = struct{}{}
	}
	for method, entry := range rpc.MethodRegistry {
		if _, ok := seen[method]; ok {
			continue
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Method < entries[j].Method
	})
	return entries
}

func inputSchema(entry rpc.MethodEntry) map[string]any {
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": true,
		"properties": map[string]any{
			"repository_id": map[string]any{"type": "string"},
		},
	}
	if entry.RepositoryScopeMode == rpc.ScopeSingleRepo {
		schema["required"] = []string{"repository_id"}
	}
	return schema
}

func isInternal(method string) bool {
	return method == "daemon.hello" || method == "daemon.describe"
}

func isHiddenProductionTool(method string) bool {
	switch method {
	case "workflow.validate",
		"workflow.plan",
		"workflow.graph",
		"workflow.templates.list",
		"workflow.templates.show",
		"workflow.init",
		"workflow.generate",
		"workflow.upgrade":
		return true
	default:
		return false
	}
}
