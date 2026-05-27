package rules

import (
	"regexp"

	"safeskill/internal/types"
)

type ShellExecRule struct{}

func (r ShellExecRule) Name() string     { return "ShellExec" }
func (r ShellExecRule) Severity() int    { return types.SeverityCritical }

func (r ShellExecRule) Check(content string) (bool, string) {
	patterns := []string{
		`curl\s.*\|.*\s?(?:ba)?sh`,
		`wget\s.*\|.*\s?(?:ba)?sh`,
	}
	for _, p := range patterns {
		if matched, _ := regexp.MatchString(p, content); matched {
			return true, "pipes curl/wget to shell"
		}
	}
	return false, ""
}
