package rules

import (
	"regexp"

	"safeskill/internal/types"
)

type ShellExecRule struct{}

func (r ShellExecRule) Name() string     { return "ShellExec" }
func (r ShellExecRule) Severity() int    { return types.SeverityHigh }

func (r ShellExecRule) Check(content string) (bool, string) {
	patterns := []string{
		`curl\s.*\|.*\s?(?:ba)?sh`,
		`wget\s.*\|.*\s?(?:ba)?sh`,
		`\bexec\s*\(`,
		`\bspawn\s*\(`,
		`child_process`,
	}
	for _, p := range patterns {
		if matched, _ := regexp.MatchString(p, content); matched {
			return true, "executes shell commands"
		}
	}
	return false, ""
}
