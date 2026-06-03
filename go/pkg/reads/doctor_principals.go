package reads

import (
	"context"

	"github.com/halbritt/striatum/go/pkg/admin"
	"github.com/halbritt/striatum/go/pkg/db"
)

// principalsDoctorBlock returns the `daemon doctor` block describing the
// configured principals (RFC 0107) and each one's effective capability/repo
// scope, so the operator can see who can do what on which repositories of this
// self-hosted daemon.
//
// Privacy-safe: it surfaces principal ids, kinds, display names, client counts,
// granted repositories, and capability scope only — never token material (the
// principal model sits above the clients/client_capabilities substrate, and
// ListPrincipals never selects token_hash/token_salt). Like the other doctor
// blocks it never fails closed: a query error is captured in "errors" so doctor
// itself still returns.
func principalsDoctorBlock(ctx context.Context, runner db.Runner) map[string]any {
	if runner == nil {
		return map[string]any{"configured": false}
	}
	scopes, err := admin.ListPrincipals(ctx, runner)
	if err != nil {
		return map[string]any{
			"configured": true,
			"errors":     []string{"principals.list_failed: " + err.Error()},
		}
	}
	items := make([]map[string]any, 0, len(scopes))
	for _, scope := range scopes {
		capabilities := make([]map[string]any, 0, len(scope.Capabilities))
		for _, capability := range scope.Capabilities {
			entry := map[string]any{
				"capability":    string(capability.Capability),
				"session_bound": capability.SessionBound,
			}
			if capability.RepositoryID != nil {
				entry["repository_id"] = *capability.RepositoryID
			} else {
				entry["repository_id"] = nil
			}
			capabilities = append(capabilities, entry)
		}
		repositories := scope.Repositories
		if repositories == nil {
			repositories = []string{}
		}
		items = append(items, map[string]any{
			"principal_id": scope.PrincipalID,
			"kind":         scope.Kind,
			"display_name": scope.DisplayName,
			"disabled":     scope.Disabled,
			"client_count": scope.ClientCount,
			"repositories": repositories,
			"capabilities": capabilities,
		})
	}
	return map[string]any{
		"configured": true,
		"count":      len(items),
		"principals": items,
		"errors":     []string{},
	}
}
