package rules

import (
	"regexp"

	"safeskill/internal/types"
)

type NetworkRule struct{}

func (r NetworkRule) Name() string     { return "NetworkAccess" }
func (r NetworkRule) Severity() int    { return types.SeverityMedium }

func (r NetworkRule) Check(content string) (bool, string) {
	patterns := []string{
		`\bfetch\s*\(`,
		`\baxios\b`,
		`XMLHttpRequest`,
		`https?://[^\s"'\)]+`,
		`require\(['"]https?`,
		`require\(['"]net['"]`,
	}
	for _, p := range patterns {
		if matched, _ := regexp.MatchString(p, content); matched {
			return true, "makes external network requests"
		}
	}
	return false, ""
}
