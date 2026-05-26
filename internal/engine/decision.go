package engine

const (
	StatusSafe    = "SAFE"
	StatusWarning = "WARNING"
	StatusBlocked = "BLOCKED"
)

func Classify(score int) string {
	if score < 30 {
		return StatusSafe
	}
	if score < 70 {
		return StatusWarning
	}
	return StatusBlocked
}
