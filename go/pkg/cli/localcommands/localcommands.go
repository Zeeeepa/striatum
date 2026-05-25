package localcommands

type Command struct {
	Command    string
	Subcommand string
	Rationale  string
}

var commands = []Command{
	{Command: "workflow", Subcommand: "validate", Rationale: "local workflow-authoring validation reads the workflow file before daemon state exists"},
	{Command: "skills", Subcommand: "install", Rationale: "skills install renders embedded skill templates into the user/project skills dir; it writes no daemon state"},
	{Command: "plugin", Subcommand: "install", Rationale: "plugin install renders embedded plugin bundles into the user/project plugin dir; it writes no daemon state"},
	{Command: "plugin", Subcommand: "uninstall", Rationale: "plugin uninstall removes a previously rendered bundle by reading its on-disk manifest; it writes no daemon state"},
}

func All() []Command {
	return append([]Command(nil), commands...)
}

func Lookup(args []string) (Command, bool) {
	if len(args) < 2 {
		return Command{}, false
	}
	for _, command := range commands {
		if args[0] == command.Command && args[1] == command.Subcommand {
			return command, true
		}
	}
	return Command{}, false
}
