package records

import "strings"

type Problem struct {
	Field   string
	Message string
}

type ValidationError struct {
	Problems []Problem
}

func (e ValidationError) Error() string {
	parts := make([]string, 0, len(e.Problems))
	for _, problem := range e.Problems {
		if problem.Field == "" {
			parts = append(parts, problem.Message)
			continue
		}
		parts = append(parts, problem.Field+": "+problem.Message)
	}
	return "invalid record docket: " + strings.Join(parts, "; ")
}
