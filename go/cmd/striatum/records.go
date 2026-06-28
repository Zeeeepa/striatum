package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/halbritt/striatum/go/pkg/cli/routes"
	recordsinventory "github.com/halbritt/striatum/go/pkg/records/inventory"
)

func runRecords(args []string, stdout io.Writer, stderr io.Writer, repoRootOverride string) int {
	if len(args) == 0 || routes.IsHelpArg(args[0]) {
		out := stdout
		if len(args) == 0 {
			out = stderr
		}
		printRecordsHelp(out)
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	if args[0] != "migration" {
		_, _ = fmt.Fprintf(stderr, "unknown records command: %s\n", args[0])
		return 2
	}
	if len(args) == 1 || routes.IsHelpArg(args[1]) {
		out := stdout
		if len(args) == 1 {
			out = stderr
		}
		printRecordsMigrationHelp(out)
		if len(args) == 1 {
			return 2
		}
		return 0
	}
	if args[1] != "inventory" {
		_, _ = fmt.Fprintf(stderr, "unknown records migration command: %s\n", args[1])
		return 2
	}
	return runRecordsMigrationInventory(args[2:], stdout, stderr, repoRootOverride)
}

func runRecordsMigrationInventory(args []string, stdout io.Writer, stderr io.Writer, repoRootOverride string) int {
	for _, arg := range args {
		if routes.IsHelpArg(arg) {
			printRecordsInventoryHelp(stdout)
			return 0
		}
	}
	roots, err := parseRecordsInventoryArgs(args)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err.Error())
		return 2
	}
	if repoRootOverride == "" {
		repoRootOverride, err = os.Getwd()
		if err != nil {
			_, _ = fmt.Fprintln(stderr, err.Error())
			return 1
		}
	}
	manifest, err := recordsinventory.Run(context.Background(), recordsinventory.Options{
		RepoRoot: repoRootOverride,
		Roots:    roots,
	})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err.Error())
		return 1
	}
	encoder := json.NewEncoder(stdout)
	if err := encoder.Encode(manifest); err != nil {
		_, _ = fmt.Fprintln(stderr, err.Error())
		return 1
	}
	return 0
}

func parseRecordsInventoryArgs(args []string) ([]string, error) {
	roots := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		key, value, hasValue := strings.Cut(arg, "=")
		switch key {
		case "--root":
			if !hasValue {
				if i+1 >= len(args) {
					return nil, fmt.Errorf("--root requires a value")
				}
				value = args[i+1]
				i++
			}
			roots = append(roots, value)
		case "--json":
			parsed, err := optionalBool(value, hasValue)
			if err != nil {
				return nil, fmt.Errorf("--json must be a boolean")
			}
			if !parsed {
				return nil, fmt.Errorf("records migration inventory always emits JSON; --json=false is not supported")
			}
		default:
			if strings.HasPrefix(arg, "-") {
				return nil, fmt.Errorf("unknown records migration inventory flag: %s", arg)
			}
			roots = append(roots, arg)
		}
	}
	return roots, nil
}

func printRecordsHelp(out io.Writer) {
	_, _ = fmt.Fprintln(out, "usage: striatum records migration inventory [--root path]...")
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "Commands:")
	_, _ = fmt.Fprintln(out, "  migration inventory   read-only historical records inventory")
}

func printRecordsMigrationHelp(out io.Writer) {
	_, _ = fmt.Fprintln(out, "usage: striatum records migration inventory [--root path]...")
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "Commands:")
	_, _ = fmt.Fprintln(out, "  inventory   emit a deterministic JSON manifest for historical record roots")
}

func printRecordsInventoryHelp(out io.Writer) {
	_, _ = fmt.Fprintln(out, "usage: striatum records migration inventory [--root path]... [path ...]")
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "Read-only. Walks historical record roots and emits deterministic JSON entries.")
	_, _ = fmt.Fprintln(out, "Default roots: docs/operator, docs/records/audits, docs/records/_frozen, docs/dogfood, dogfoods")
	_, _ = fmt.Fprintln(out, "Repeat --root to override the defaults. Positional paths are also treated as roots.")
}
