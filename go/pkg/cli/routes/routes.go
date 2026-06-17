// Package routes contains generated CLI-to-daemon route metadata.
package routes

//go:generate go run ../routergen -mode routes -contract ../../../../contracts/daemon_methods.json -out routes_generated.go

type Route struct {
	Command             string
	Subcommand          string
	Method              string
	ParamsGroup         string
	RequiredCapability  string
	RepositoryScopeMode string
	Deprecated          bool
}

func All() []Route {
	return append([]Route(nil), generatedRoutes...)
}

var runtimeRoutes = []Route{
	// `session register` alias for the top-level `register-session` verb so the
	// session noun is consistent with `session close` (issue #63 F9). The
	// original top-level verb stays registered for backward compatibility.
	{Command: "session", Subcommand: "register", Method: "session.register", ParamsGroup: "register_session", RequiredCapability: "claim", RepositoryScopeMode: "single_repo", Deprecated: false},
}

func Lookup(args []string) (Route, int, bool) {
	if len(args) == 0 {
		return Route{}, 0, false
	}
	for _, route := range generatedRoutes {
		if route.Command != args[0] {
			continue
		}
		if route.Subcommand == "" {
			return route, 1, true
		}
		if len(args) > 1 && route.Subcommand == args[1] {
			return route, 2, true
		}
	}
	for _, route := range runtimeRoutes {
		if route.Command != args[0] {
			continue
		}
		if route.Subcommand == "" {
			return route, 1, true
		}
		if len(args) > 1 && route.Subcommand == args[1] {
			return route, 2, true
		}
	}
	return Route{}, 0, false
}
