package engine

import "safeskill/internal/types"

func ApplyBoosts(signals []types.Signal, score int) int {
	has := func(name string) bool {
		for _, s := range signals {
			if s.Rule == name {
				return true
			}
		}
		return false
	}

	if has("Obfuscation") && has("DynamicEval") {
		score += 30
	}
	if has("NetworkAccess") && has("EnvAccess") {
		score += 25
	}
	if has("PostinstallHook") && has("ShellExec") {
		score += 40
	}

	for _, s := range signals {
		if s.Severity >= types.SeverityCritical {
			return 100
		}
	}

	return score
}
