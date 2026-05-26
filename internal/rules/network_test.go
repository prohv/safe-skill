package rules

import "testing"

func TestNetworkRule(t *testing.T) {
	r := NetworkRule{}

	t.Run("name", func(t *testing.T) {
		if r.Name() != "NetworkAccess" {
			t.Errorf("Name() = %s, want NetworkAccess", r.Name())
		}
	})

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"fetch call", `fetch('https://api.example.com')`, true},
		{"axios usage", `const axios = require('axios')`, true},
		{"XMLHttpRequest", `new XMLHttpRequest()`, true},
		{"http URL", `const url = "http://evil.com/payload"`, true},
		{"require http", `require('https')`, true},
		{"require net", `require('net')`, true},
		{"clean imports", `import { something } from './local'`, false},
		{"empty string", ``, false},
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
