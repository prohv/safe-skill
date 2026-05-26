package rules

import (
	"strings"
	"testing"
)

func TestObfuscationRule(t *testing.T) {
	r := ObfuscationRule{}

	t.Run("name", func(t *testing.T) {
		if r.Name() != "Obfuscation" {
			t.Errorf("Name() = %s, want Obfuscation", r.Name())
		}
	})

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			"base64 string",
			"dGhpcyBpcyBhIGJhc2U2NCBlbmNvZGVkIHN0cmluZyB0aGF0IGlzIGxvbmcgZW5vdWdo",
			true,
		},
		{
			"over 500 chars line",
			strings.Repeat("x", 501),
			true,
		},
		{
			"short base64-like",
			"abc123+/",
			false,
		},
		{
			"normal lines",
			"const x = 1\nconst y = 2\nconsole.log(x + y)",
			false,
		},
		{
			"empty string",
			"",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, _ := r.Check(tt.content)
			if matched != tt.want {
				t.Errorf("Check() = %v, want %v", matched, tt.want)
			}
		})
	}
}
