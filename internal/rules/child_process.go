package rules

import (
	"regexp"

	"safeskill/internal/types"
)

type ChildProcessRule struct{}

func (r ChildProcessRule) Name() string    { return "ChildProcess" }
func (r ChildProcessRule) Severity() int   { return types.SeverityLow * 2 }

func (r ChildProcessRule) Check(content string) (bool, string) {
	patterns := []string{
		`\bexec\s*\(`,
		`\bspawn\s*\(`,
		`child_process`,
	}
	for _, p := range patterns {
		if matched, _ := regexp.MatchString(p, content); matched {
			return true, "uses child process execution"
		}
	}
	return false, ""
}
