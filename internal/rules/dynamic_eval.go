package rules

import (
	"regexp"

	"safeskill/internal/types"
)

type DynamicEvalRule struct{}

func (r DynamicEvalRule) Name() string     { return "DynamicEval" }
func (r DynamicEvalRule) Severity() int    { return types.SeverityHigh }

func (r DynamicEvalRule) Check(content string) (bool, string) {
	patterns := []string{
		`\beval\s*\(`,
		`new\s+Function\s*\(`,
	}
	for _, p := range patterns {
		if matched, _ := regexp.MatchString(p, content); matched {
			return true, "uses dynamic code evaluation"
		}
	}
	return false, ""
}
