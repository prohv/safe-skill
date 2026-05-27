package engine

import (
	"testing"

	"safeskill/internal/types"
)

func sig(name string, sev int) types.Signal {
	return types.Signal{Rule: name, Severity: sev}
}

func sigs(signals ...types.Signal) []types.Signal {
	return signals
}

func TestApplyBoosts(t *testing.T) {
	tests := []struct {
		name    string
		signals []types.Signal
		want    int
	}{
		{"no signals", sigs(), 0},
		{"one signal no boost", sigs(sig("ShellExec", 50)), 50},
		{"obfuscation + eval", sigs(sig("Obfuscation", 20), sig("DynamicEval", 50)), 100},
		{"network + env", sigs(sig("NetworkAccess", 30), sig("EnvAccess", 30)), 85},
		{"postinstall + exec", sigs(sig("PostinstallHook", 50), sig("ShellExec", 50)), 140},
		{"obfuscation alone", sigs(sig("Obfuscation", 20)), 20},
		{"eval alone", sigs(sig("DynamicEval", 50)), 50},
		{"all three pairs", sigs(
			sig("Obfuscation", 20), sig("DynamicEval", 50),
			sig("NetworkAccess", 30), sig("EnvAccess", 30),
			sig("PostinstallHook", 50), sig("ShellExec", 50),
		), 325},
		{"critical overrides all", sigs(sig("ShellExec", 80)), 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseScore := 0
			for _, s := range tt.signals {
				baseScore += s.Severity
			}
			got := ApplyBoosts(tt.signals, baseScore)
			if got != tt.want {
				t.Errorf("ApplyBoosts() = %d, want %d", got, tt.want)
			}
		})
	}
}
