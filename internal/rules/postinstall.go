package rules

import (
	"regexp"

	"safeskill/internal/types"
)

type PostinstallRule struct{}

func (r PostinstallRule) Name() string     { return "PostinstallHook" }
func (r PostinstallRule) Severity() int    { return types.SeverityHigh }

func (r PostinstallRule) Check(content string) (bool, string) {
	patterns := []string{
		`"postinstall"\s*:\s*"`,
		`"preinstall"\s*:\s*"`,
		`"install"\s*:\s*"`,
	}
	for _, p := range patterns {
		if matched, _ := regexp.MatchString(p, content); matched {
			return true, "has lifecycle install scripts"
		}
	}
	return false, ""
}
