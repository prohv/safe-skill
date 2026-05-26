package rules

import (
	"regexp"

	"safeskill/internal/types"
)

type EnvAccessRule struct{}

func (r EnvAccessRule) Name() string     { return "EnvAccess" }
func (r EnvAccessRule) Severity() int    { return types.SeverityMedium }

func (r EnvAccessRule) Check(content string) (bool, string) {
	patterns := []string{
		`process\.env`,
		`os\.environ`,
		`\bgetenv\s*\(`,
		`\$ENV\{`,
	}
	for _, p := range patterns {
		if matched, _ := regexp.MatchString(p, content); matched {
			return true, "accesses environment variables"
		}
	}
	return false, ""
}
