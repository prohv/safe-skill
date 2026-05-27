package rules

import "testing"

func TestDynamicEvalRule(t *testing.T) {
	r := DynamicEvalRule{}

	t.Run("name", func(t *testing.T) {
		if r.Name() != "DynamicEval" {
			t.Errorf("Name() = %s, want DynamicEval", r.Name())
		}
	})

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"eval call", `eval('var x = 1')`, true},
		{"eval with spaces", `eval ("bad")`, true},
		{"new Function", `new Function("return this")`, true},
		{"new Function multiline", "new\nFunction('x', 'return x')", true},
		{"evaluate variable", `const evaluation = 5`, false},
		{"function without new", `Function("return this")`, false},
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
