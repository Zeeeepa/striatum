package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/halbritt/striatum/go/pkg/cli/mutationparams"
	"github.com/halbritt/striatum/go/pkg/cli/readparams"
	"github.com/halbritt/striatum/go/pkg/cli/routes"
)

type Invoker interface {
	Invoke(context.Context, string, map[string]any) (map[string]any, error)
}

type Options struct {
	Env              []string
	Cwd              string
	Invoker          Invoker
	InvokerFactory   func(RuntimeConfig) (Invoker, error)
	ResolveRepo      bool
	ResolveRepoRoute string
	ExitCode         func(error) int
}

type RuntimeConfig struct {
	SocketPath string
	Token      string
	TokenFile  string
	DeadlineMS int
}

type globalOptions struct {
	RepoPath     string
	RepositoryID string
	SocketPath   string
	Token        string
	TokenFile    string
	DeadlineMS   int
	JSONOutput   bool
	CommandArgs  []string
}

func Run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer, options Options) int {
	globals, err := parseGlobal(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	route, consumed, ok := routes.Lookup(globals.CommandArgs)
	if !ok {
		if len(globals.CommandArgs) > 0 {
			fmt.Fprintf(stderr, "unknown command: %s\n", strings.Join(globals.CommandArgs, " "))
		} else {
			fmt.Fprintln(stderr, "usage: striatum [global options] command ...")
		}
		return 2
	}
	invoker := options.Invoker
	if invoker == nil && options.InvokerFactory != nil {
		invoker, err = options.InvokerFactory(RuntimeConfig{
			SocketPath: globals.SocketPath,
			Token:      globals.Token,
			TokenFile:  globals.TokenFile,
			DeadlineMS: globals.DeadlineMS,
		})
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
	}
	if invoker == nil {
		fmt.Fprintln(stderr, "daemon RPC invoker is not configured")
		return 1
	}
	repositoryID := globals.RepositoryID
	if repositoryID == "" {
		repositoryID = envValue(options.Env, "STRIATUM_REPOSITORY_ID")
	}
	if route.RepositoryScopeMode == "single_repo" && repositoryID == "" {
		if options.ResolveRepo {
			resolved, err := resolveRepository(ctx, invoker, globals.RepoPath, options)
			if err != nil {
				return writeError(stderr, err, options)
			}
			repositoryID = resolved
		}
	}
	params, err := buildParams(route, globals.CommandArgs[consumed:], repositoryID)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	data, err := invoker.Invoke(ctx, route.Method, params)
	if err != nil {
		return writeError(stderr, err, options)
	}
	if globals.JSONOutput {
		return writeJSON(stdout, map[string]any{"ok": true, "data": data}, stderr)
	}
	return writeJSON(stdout, data, stderr)
}

func parseGlobal(args []string) (globalOptions, error) {
	var options globalOptions
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			options.CommandArgs = append(options.CommandArgs, args[i+1:]...)
			return options, nil
		}
		if !strings.HasPrefix(arg, "--") || arg == "--" {
			options.CommandArgs = append(options.CommandArgs, args[i:]...)
			return options, nil
		}
		keyValue := strings.TrimPrefix(arg, "--")
		key, value, hasValue := strings.Cut(keyValue, "=")
		switch key {
		case "repo":
			value, hasValue, i = readGlobalValue(args, i, value, hasValue)
			if !hasValue {
				return options, fmt.Errorf("--repo requires a value")
			}
			options.RepoPath = value
		case "repository-id":
			value, hasValue, i = readGlobalValue(args, i, value, hasValue)
			if !hasValue {
				return options, fmt.Errorf("--repository-id requires a value")
			}
			options.RepositoryID = value
		case "daemon-socket":
			value, hasValue, i = readGlobalValue(args, i, value, hasValue)
			if !hasValue {
				return options, fmt.Errorf("--daemon-socket requires a value")
			}
			options.SocketPath = value
		case "capability-token":
			value, hasValue, i = readGlobalValue(args, i, value, hasValue)
			if !hasValue {
				return options, fmt.Errorf("--capability-token requires a value")
			}
			options.Token = value
		case "capability-token-file":
			value, hasValue, i = readGlobalValue(args, i, value, hasValue)
			if !hasValue {
				return options, fmt.Errorf("--capability-token-file requires a value")
			}
			options.TokenFile = value
		case "deadline-ms":
			value, hasValue, i = readGlobalValue(args, i, value, hasValue)
			if !hasValue {
				return options, fmt.Errorf("--deadline-ms requires a value")
			}
			deadline, err := strconv.Atoi(value)
			if err != nil || deadline < 0 {
				return options, fmt.Errorf("--deadline-ms must be a non-negative integer")
			}
			options.DeadlineMS = deadline
		case "json":
			options.JSONOutput = true
		default:
			options.CommandArgs = append(options.CommandArgs, args[i:]...)
			return options, nil
		}
	}
	return options, nil
}

func readGlobalValue(args []string, i int, value string, hasValue bool) (string, bool, int) {
	if hasValue {
		return value, true, i
	}
	if i+1 >= len(args) {
		return "", false, i
	}
	return args[i+1], true, i + 1
}

func buildParams(route routes.Route, args []string, repositoryID string) (map[string]any, error) {
	options := readparams.Options{RepositoryID: repositoryID}
	if route.RequiredCapability == "read" || route.RequiredCapability == "" {
		return readparams.Build(route.ParamsGroup, args, options)
	}
	return mutationparams.Build(route.ParamsGroup, args, mutationparams.Options{RepositoryID: repositoryID})
}

func resolveRepository(ctx context.Context, invoker Invoker, repoPath string, options Options) (string, error) {
	if repoPath == "" {
		if options.Cwd != "" {
			repoPath = options.Cwd
		} else if cwd, err := os.Getwd(); err == nil {
			repoPath = cwd
		}
	}
	route := options.ResolveRepoRoute
	if route == "" {
		route = "repo.resolve"
	}
	data, err := invoker.Invoke(ctx, route, map[string]any{"path": repoPath})
	if err != nil {
		return "", err
	}
	repositoryID, _ := data["repository_id"].(string)
	if repositoryID == "" {
		return "", fmt.Errorf("repo.resolve response did not include repository_id")
	}
	return repositoryID, nil
}

func writeError(stderr io.Writer, err error, options Options) int {
	fmt.Fprintln(stderr, err.Error())
	if options.ExitCode != nil {
		return options.ExitCode(err)
	}
	return 1
}

func writeJSON(stdout io.Writer, payload any, stderr io.Writer) int {
	encoded, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	fmt.Fprintln(stdout, string(encoded))
	return 0
}

func envValue(env []string, key string) string {
	if env == nil {
		env = os.Environ()
	}
	for _, item := range env {
		name, value, ok := strings.Cut(item, "=")
		if ok && name == key {
			return value
		}
	}
	return ""
}
