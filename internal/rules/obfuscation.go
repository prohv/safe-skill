package rules

import (
	"regexp"
	"strings"

	"safeskill/internal/types"
)

type ObfuscationRule struct{}

func (r ObfuscationRule) Name() string     { return "Obfuscation" }
func (r ObfuscationRule) Severity() int    { return types.SeverityMedium }

func (r ObfuscationRule) Check(content string) (bool, string) {
	if matched, _ := regexp.MatchString(`[A-Za-z0-9+/]{40,}={0,2}`, content); matched {
		return true, "contains base64-encoded data"
	}

	for _, line := range strings.Split(content, "\n") {
		if len(line) > 500 {
			return true, "contains heavily obfuscated lines"
		}
	}

	return false, ""
}
