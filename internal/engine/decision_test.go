package engine

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		name  string
		score int
		want  string
	}{
		{"safe zero", 0, StatusSafe},
		{"safe low", 15, StatusSafe},
		{"safe boundary", 29, StatusSafe},
		{"warn low", 30, StatusWarning},
		{"warn mid", 50, StatusWarning},
		{"warn boundary", 69, StatusWarning},
		{"blocked low", 70, StatusBlocked},
		{"blocked high", 150, StatusBlocked},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.score)
			if got != tt.want {
				t.Errorf("Classify(%d) = %s, want %s", tt.score, got, tt.want)
			}
		})
	}
}
