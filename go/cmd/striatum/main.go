package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/halbritt/striatum/go/pkg/cli/dispatch"
	"github.com/halbritt/striatum/go/pkg/cli/localcommands"
	"github.com/halbritt/striatum/go/pkg/cli/rpcclient"
	cliskills "github.com/halbritt/striatum/go/pkg/cli/skills"
	"github.com/halbritt/striatum/go/pkg/workflowauthoring"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	globals, err := parseLeadingGlobals(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	if _, ok := localcommands.Lookup(globals.CommandArgs); ok {
		commandArgs := globals.CommandArgs
		switch commandArgs[0] {
		case "skills", "plugin":
			runArgs := append([]string(nil), commandArgs...)
			if globals.JSONOutput && !containsFlag(runArgs, "--json") {
				runArgs = append(runArgs, "--json")
			}
			return cliskills.Run(runArgs, stdout, stderr, globals.RepoPath, version)
		default:
			workflowArgs := commandArgs[1:]
			if globals.JSONOutput && !containsFlag(workflowArgs, "--json") {
				workflowArgs = append([]string{workflowArgs[0], "--json"}, workflowArgs[1:]...)
			}
			return runWorkflow(workflowArgs, stdout, stderr, globals.RepoPath)
		}
	}
	switch args[0] {
	case "-h", "--help", "help":
		usage(stdout)
		return 0
	case "--version":
		fmt.Fprintln(stdout, version)
		return 0
	default:
		return runDaemonRoute(args, stdout, stderr)
	}
}

func usage(out io.Writer) {
	fmt.Fprintln(out, "usage: striatum [--version] [--repo path|--repository-id id] command ...")
}

func runDaemonRoute(args []string, stdout io.Writer, stderr io.Writer) int {
	return dispatch.Run(context.Background(), args, stdout, stderr, dispatch.Options{
		Env:         os.Environ(),
		ResolveRepo: true,
		ExitCode:    rpcclient.ExitCode,
		InvokerFactory: func(runtime dispatch.RuntimeConfig) (dispatch.Invoker, error) {
			config, err := rpcclient.ResolveConfig(os.Environ(), runtime.SocketPath, runtime.Token, runtime.TokenFile, runtime.DeadlineMS)
			if err != nil {
				return nil, err
			}
			return rpcclient.Client{Config: config}, nil
		},
	})
}

type leadingGlobals struct {
	CommandArgs []string
	RepoPath    string
	JSONOutput  bool
}

func parseLeadingGlobals(args []string) (leadingGlobals, error) {
	var globals leadingGlobals
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			globals.CommandArgs = args[i+1:]
			return globals, nil
		}
		if !strings.HasPrefix(arg, "--") {
			globals.CommandArgs = args[i:]
			return globals, nil
		}
		keyValue := strings.TrimPrefix(arg, "--")
		key, value, hasValue := strings.Cut(keyValue, "=")
		switch key {
		case "json":
			globals.JSONOutput = true
		case "repo", "repository-id", "daemon-socket", "capability-token", "capability-token-file", "deadline-ms":
			if !hasValue {
				if i+1 >= len(args) {
					return leadingGlobals{}, fmt.Errorf("--%s requires a value", key)
				}
				value = args[i+1]
				i++
			}
			if key == "repo" {
				globals.RepoPath = value
			}
		default:
			globals.CommandArgs = args[i:]
			return globals, nil
		}
	}
	return globals, nil
}

func containsFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag {
			return true
		}
	}
	return false
}

func runWorkflow(args []string, stdout io.Writer, stderr io.Writer, repoRootOverride string) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: striatum workflow {validate} ...")
		return 2
	}
	switch args[0] {
	case "validate":
		return runWorkflowValidate(args[1:], stdout, stderr, repoRootOverride)
	default:
		fmt.Fprintf(stderr, "unknown workflow command: %s\n", args[0])
		return 2
	}
}

func runWorkflowValidate(args []string, stdout io.Writer, stderr io.Writer, repoRootOverride string) int {
	allowSameModel, jsonOutput, paths, err := parseWorkflowValidateArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	if len(paths) != 1 {
		fmt.Fprintln(stderr, "usage: striatum workflow validate [--allow-same-model-pairing] [--json] path")
		return 2
	}
	repoRoot := repoRootOverride
	if repoRoot == "" {
		repoRoot, err = os.Getwd()
		if err != nil {
			return outputWorkflowValidateError(stdout, stderr, jsonOutput, "cwd_error", err, 1)
		}
	}
	workflow, _, err := workflowauthoring.LoadFile(repoRoot, paths[0])
	if err != nil {
		return outputWorkflowValidateError(stdout, stderr, jsonOutput, "workflow_invalid", err, 8)
	}
	if !allowSameModel {
		if err := refuseSameModelLint(workflow); err != nil {
			return outputWorkflowValidateError(stdout, stderr, jsonOutput, "workflow_lint_refused", err, 8)
		}
	}
	if jsonOutput {
		return writeJSON(stdout, map[string]any{
			"ok": true,
			"data": map[string]any{
				"valid":       true,
				"workflow_id": workflow["workflow_id"],
			},
		}, stderr)
	}
	fmt.Fprintln(stdout, "valid")
	return 0
}

func parseWorkflowValidateArgs(args []string) (bool, bool, []string, error) {
	allowSameModel := false
	jsonOutput := false
	paths := []string{}
	for _, arg := range args {
		key, value, hasValue := strings.Cut(arg, "=")
		switch key {
		case "--allow-same-model-pairing":
			parsed, err := optionalBool(value, hasValue)
			if err != nil {
				return false, false, nil, fmt.Errorf("--allow-same-model-pairing must be a boolean")
			}
			allowSameModel = parsed
		case "--json":
			parsed, err := optionalBool(value, hasValue)
			if err != nil {
				return false, false, nil, fmt.Errorf("--json must be a boolean")
			}
			jsonOutput = parsed
		default:
			if strings.HasPrefix(arg, "-") {
				return false, false, nil, fmt.Errorf("unknown workflow validate flag: %s", arg)
			}
			paths = append(paths, arg)
		}
	}
	return allowSameModel, jsonOutput, paths, nil
}

func optionalBool(value string, hasValue bool) (bool, error) {
	if !hasValue {
		return true, nil
	}
	return strconv.ParseBool(value)
}

func refuseSameModelLint(workflow map[string]any) error {
	result, err := workflowauthoring.Lint(workflow)
	if err != nil {
		return err
	}
	for _, warning := range anySlice(result["warnings"]) {
		item, ok := warning.(map[string]any)
		if !ok {
			continue
		}
		rule, _ := item["rule"].(string)
		if rule == "same_model_review_pair" || rule == "same_model_revision_cycle" {
			message, _ := item["message"].(string)
			if strings.TrimSpace(message) == "" {
				message = "same-model review pairing refused"
			}
			return errors.New(message)
		}
	}
	return nil
}

func outputWorkflowValidateError(stdout io.Writer, stderr io.Writer, jsonOutput bool, code string, err error, exitCode int) int {
	if jsonOutput {
		_ = writeJSON(stdout, map[string]any{
			"ok": false,
			"error": map[string]any{
				"code":    code,
				"message": err.Error(),
			},
		}, stderr)
		return exitCode
	}
	fmt.Fprintln(stderr, err.Error())
	return exitCode
}

func writeJSON(stdout io.Writer, payload map[string]any, stderr io.Writer) int {
	encoded, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	fmt.Fprintln(stdout, string(encoded))
	return 0
}

func anySlice(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case []map[string]any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, item)
		}
		return result
	default:
		return nil
	}
}
