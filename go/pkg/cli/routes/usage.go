package routes

import (
	"fmt"
	"sort"
	"strings"
)

// Param describes one positional or flag input for a CLI verb, so `--help`
// can list required AND optional inputs instead of forcing the operator to
// discover them as runtime "<method> requires <param>" errors (issue #63 F9).
type Param struct {
	// Name is the canonical wire/flag name in kebab-case (e.g. "session-id",
	// "reason", "capability"). Positionals reuse the same name.
	Name string
	// Positional is true when the value may be supplied positionally (it can
	// always also be supplied as --name value).
	Positional bool
	// Required is true when the daemon rejects the call without this input.
	Required bool
	// Repeatable is true when the flag may be passed more than once
	// (e.g. --capability).
	Repeatable bool
	// Bool is true when the flag is a presence flag that takes no value
	// (e.g. --fresh).
	Bool bool
	// Values lists the accepted enum values when the input is constrained
	// (e.g. action: continue|cancel).
	Values []string
	// Help is a short human description.
	Help string
}

// Usage is the discoverability descriptor for a single CLI verb.
type Usage struct {
	Params []Param
	// Notes are extra free-form lines shown under the flag list (e.g. to call
	// out the register-session naming history).
	Notes []string
}

// usageByGroup maps a route ParamsGroup to its discoverability descriptor.
// Only groups with an entry here render a full `--help`; others fall back to a
// generic synopsis derived from the route metadata.
var usageByGroup = map[string]Usage{
	"repo_add": {
		Params: []Param{
			{Name: "path", Positional: true, Required: true, Help: "target repository path to register"},
			{Name: "init", Bool: true, Help: "create .striatum/scratch and add .striatum/ to .gitignore when registering a fresh target repo"},
			{Name: "display-name", Help: "operator-facing repository name; defaults to the directory basename"},
			{Name: "no-migrate", Bool: true, Help: "accepted compatibility flag; production registration never imports retired SQLite state"},
		},
	},
	"register_session": {
		Params: []Param{
			{Name: "run-id", Positional: true, Required: true, Help: "run that owns the session"},
			{Name: "role", Positional: true, Required: true, Help: "workflow role id (e.g. author, reviewer)"},
			{Name: "lane", Positional: true, Required: true, Help: "workflow lane id"},
			{Name: "capability", Repeatable: true, Help: "grant a capability; repeat per capability (defaults to the lane's declared capabilities). Note: the flag is --capability (singular), not --capabilities"},
			{Name: "fresh", Bool: true, Help: "register a fresh-context session"},
			{Name: "replace", Bool: true, Help: "atomically close any active session on this (run, role, lane) slot and transfer its leases before registering (alias: --force). Without it a duplicate active session is refused with the id to close, and parallel same-(role,lane) jobs each keep a distinct active session."},
			{Name: "force", Bool: true, Help: "alias for --replace"},
			{Name: "parent-session-id", Help: "parent session id for derived/continued context"},
			{Name: "operator-label", Help: "operator-visible label for the session"},
			{Name: "force-non-fresh", Bool: true, Help: "allow a non-fresh reviewer when the workflow declares reviewer_context_policy: fresh (requires --reason)"},
			{Name: "reason", Help: "justification recorded with --force-non-fresh"},
		},
		Notes: []string{
			"Aliases: `striatum session register ...` resolves to the same method (session.register).",
			"Parallel same-(role, lane) jobs (declared parallelism, disjoint scopes) each register their own fresh session and both stay active; the second registration no longer supersedes the first.",
		},
	},
	"session_close": {
		Params: []Param{
			{Name: "session-id", Positional: true, Required: true, Help: "session to close"},
			{Name: "reason", Required: true, Help: "why the session is being closed (the daemon rejects an empty reason: \"session close reason must not be empty\")"},
			{Name: "requeue-job", Bool: true, Help: "return this session's in-flight job to the queue on the same attempt (no attempt bump, no downstream reset) so a fresh lane can pick it up (#121)"},
		},
	},
	"checkpoint_resolve": {
		Params: []Param{
			{Name: "blocker-id", Positional: true, Required: true, Help: "human_checkpoint blocker to resolve"},
			{Name: "action", Positional: true, Required: true, Values: []string{"continue", "cancel", "override"}, Help: "continue resolves the checkpoint and proceeds; cancel cancels the gated work; override accepts a revision_routing checkpoint as superseded by a recorded decision (requires --decision-id)"},
			{Name: "decision-id", Help: "logical name of a run-level decision artifact to attach as the resolution rationale; required for override"},
		},
	},
	"supervise_start": {
		Params: []Param{
			{Name: "session-id", Positional: true, Required: true, Help: "session whose lane to supervise"},
			{Name: "replace", Bool: true, Help: "supersede any stale active supervisor for this session instead of refusing with a conflict error"},
		},
	},
	"supervise_send": {
		Params: []Param{
			{Name: "session-id", Positional: true, Required: true, Help: "supervised session"},
			{Name: "packet-id", Positional: true, Required: true, Help: "work packet to deliver to the lane's stdin"},
		},
	},
	"supervise_stop": {
		Params: []Param{
			{Name: "session-id", Positional: true, Required: true, Help: "supervised session to stop"},
			{Name: "reason", Required: true, Help: "why the supervisor is being stopped (recorded as stop_reason)"},
		},
	},
	"supervise_status": {
		Params: []Param{
			{Name: "session-id", Positional: true, Required: true, Help: "supervised session to report on"},
		},
	},
	"supervise_list": {
		Params: []Param{
			{Name: "run-id", Positional: true, Required: true, Help: "run whose supervisors to list"},
			{Name: "state", Help: "filter by supervisor state (e.g. attached, stopped)"},
		},
	},
	"supervise_rebridge": {
		Params: []Param{
			{Name: "session-id", Positional: true, Required: true, Help: "supervised session to re-attach delivery for"},
		},
	},
}

// UsageFor returns the discoverability descriptor for a route's ParamsGroup,
// if one is registered.
func UsageFor(group string) (Usage, bool) {
	usage, ok := usageByGroup[group]
	return usage, ok
}

// IsHelpArg reports whether arg requests help for the verb.
func IsHelpArg(arg string) bool {
	switch arg {
	case "-h", "--help", "help":
		return true
	default:
		return false
	}
}

// RenderHelp formats a verb's usage. command is the resolved invocation prefix
// (e.g. "supervise start" or "register-session").
func (r Route) RenderHelp() string {
	command := r.Command
	if r.Subcommand != "" {
		command += " " + r.Subcommand
	}
	usage, ok := UsageFor(r.ParamsGroup)

	var b strings.Builder
	synopsis := "usage: striatum " + command
	if ok {
		for _, p := range usage.Params {
			synopsis += " " + p.synopsisToken()
		}
	} else {
		synopsis += " [args ...]"
	}
	b.WriteString(synopsis)
	b.WriteString("\n")

	fmt.Fprintf(&b, "method: %s", r.Method)
	if r.RequiredCapability != "" {
		fmt.Fprintf(&b, "  (capability: %s)", r.RequiredCapability)
	}
	b.WriteString("\n")

	if !ok {
		b.WriteString("\nflags are derived from the daemon method; see docs/reference/command-authority-matrix.md\n")
		return b.String()
	}

	required := []Param{}
	optional := []Param{}
	for _, p := range usage.Params {
		if p.Required {
			required = append(required, p)
		} else {
			optional = append(optional, p)
		}
	}
	if len(required) > 0 {
		b.WriteString("\nrequired:\n")
		for _, p := range required {
			b.WriteString(p.helpLine())
		}
	}
	if len(optional) > 0 {
		b.WriteString("\noptional:\n")
		for _, p := range optional {
			b.WriteString(p.helpLine())
		}
	}
	for _, note := range usage.Notes {
		b.WriteString("\n")
		b.WriteString(note)
		b.WriteString("\n")
	}
	return b.String()
}

func (p Param) synopsisToken() string {
	if p.Positional {
		if p.Required {
			return "<" + p.Name + ">"
		}
		return "[" + p.Name + "]"
	}
	token := "--" + p.Name
	switch {
	case p.Bool:
		// presence flag: no value token
	case len(p.Values) > 0:
		token += " " + strings.Join(p.Values, "|")
	default:
		token += " <value>"
	}
	if p.Repeatable {
		token += " ..."
	}
	if p.Required {
		return token
	}
	return "[" + token + "]"
}

func (p Param) helpLine() string {
	name := "--" + p.Name
	if p.Positional {
		name = "<" + p.Name + "> | " + name
	}
	suffix := ""
	if p.Bool {
		suffix += " (flag)"
	}
	if p.Repeatable {
		suffix += " (repeatable)"
	}
	if len(p.Values) > 0 {
		suffix += " {" + strings.Join(p.Values, "|") + "}"
	}
	return fmt.Sprintf("  %-26s %s%s\n", name, p.Help, suffix)
}

// HelpGroups returns the sorted set of ParamsGroups with registered usage; it
// keeps tests stable and lets tooling enumerate documented verbs.
func HelpGroups() []string {
	groups := make([]string, 0, len(usageByGroup))
	for group := range usageByGroup {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return groups
}
