package cli

import (
	"fmt"
	"os"
)

func Red(s string) string {
	return "\033[31m" + s + "\033[0m"
}

func Yellow(s string) string {
	return "\033[33m" + s + "\033[0m"
}

func Green(s string) string {
	return "\033[32m" + s + "\033[0m"
}

func Bold(s string) string {
	return "\033[1m" + s + "\033[0m"
}

func ColorStatus(status string) string {
	switch status {
	case "SAFE":
		return Green(status)
	case "WARNING":
		return Yellow(status)
	case "BLOCKED":
		return Red(status)
	default:
		return status
	}
}

func IsTerminal() bool {
	stat, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

func FormatBlocked(risk int, pkg string, summary string) string {
	return fmt.Sprintf("\n%s (Risk: %d) — %s — %s",
		Red("BLOCKED"), risk, Bold(pkg), summary)
}
