package mutations

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDaemonMutationGitInvocationsDoNotUseCheckoutOrWorkingTreeMerge(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob mutation files: %v", err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			if forbiddenGitCall(call) {
				pos := fset.Position(call.Pos())
				t.Fatalf("%s invokes forbidden working-tree git command; daemon mutation paths must use ref/plumbing operations instead", pos)
			}
			return true
		})
	}
}

func forbiddenGitCall(call *ast.CallExpr) bool {
	name := callName(call.Fun)
	switch name {
	case "exec.Command", "exec.CommandContext":
		if len(call.Args) < 2 || stringLiteral(call.Args[0]) != "git" {
			return false
		}
		return forbiddenGitSubcommand(stringLiteral(call.Args[1]))
	case "runGitWorktreeCommand", "integrateGit":
		if len(call.Args) < 3 {
			return false
		}
		return forbiddenGitSubcommand(stringLiteral(call.Args[2]))
	default:
		return false
	}
}

func forbiddenGitSubcommand(subcommand string) bool {
	return subcommand == "checkout" || subcommand == "merge"
}

func callName(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		if receiver, ok := value.X.(*ast.Ident); ok {
			return receiver.Name + "." + value.Sel.Name
		}
	}
	return ""
}

func stringLiteral(expr ast.Expr) string {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return ""
	}
	return value
}
