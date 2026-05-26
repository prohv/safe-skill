package rules

import "testing"

func TestEnvAccessRule(t *testing.T) {
	r := EnvAccessRule{}

	t.Run("name", func(t *testing.T) {
		if r.Name() != "EnvAccess" {
			t.Errorf("Name() = %s, want EnvAccess", r.Name())
		}
	})

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"process.env", `const token = process.env.SECRET`, true},
		{"process.env assignment", `process.env.NODE_ENV = 'prod'`, true},
		{"os.environ", `import os; os.environ['HOME']`, true},
		{"getenv call", `getenv('DATABASE_URL')`, true},
		{"clean variable", `const env = 'production'`, false},
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
