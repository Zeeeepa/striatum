package mutationparams

import "github.com/halbritt/striatum/go/pkg/cli/params"

type Options = params.Options

func Build(group string, args []string, options Options) (map[string]any, error) {
	return params.Build(group, args, options)
}
