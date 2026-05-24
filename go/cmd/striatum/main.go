package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

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
	switch args[0] {
	case "-h", "--help", "help":
		usage(stdout)
		return 0
	case "--version":
		fmt.Fprintln(stdout, version)
		return 0
	case "workflow":
		return runWorkflow(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		usage(stderr)
		return 2
	}
}

func usage(out io.Writer) {
	fmt.Fprintln(out, "usage: striatum [--version] {workflow} ...")
}

func runWorkflow(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: striatum workflow {validate} ...")
		return 2
	}
	switch args[0] {
	case "validate":
		return runWorkflowValidate(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown workflow command: %s\n", args[0])
		return 2
	}
}

func runWorkflowValidate(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("workflow validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	allowSameModel := fs.Bool("allow-same-model-pairing", false, "accept same-model implementer/reviewer pairings")
	jsonOutput := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: striatum workflow validate [--allow-same-model-pairing] [--json] path")
		return 2
	}
	repoRoot, err := os.Getwd()
	if err != nil {
		return outputWorkflowValidateError(stdout, stderr, *jsonOutput, "cwd_error", err, 1)
	}
	workflow, _, err := workflowauthoring.LoadFile(repoRoot, fs.Arg(0))
	if err != nil {
		return outputWorkflowValidateError(stdout, stderr, *jsonOutput, "workflow_invalid", err, 8)
	}
	if !*allowSameModel {
		if err := refuseSameModelLint(workflow); err != nil {
			return outputWorkflowValidateError(stdout, stderr, *jsonOutput, "workflow_lint_refused", err, 8)
		}
	}
	if *jsonOutput {
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
