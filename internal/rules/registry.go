package rules

import "safeskill/internal/types"

func BuiltinRules() []types.Rule {
	return []types.Rule{
		ShellExecRule{},
		DynamicEvalRule{},
		NetworkRule{},
		EnvAccessRule{},
		ObfuscationRule{},
	}
}
