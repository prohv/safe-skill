package types

type Rule interface {
	Name() string
	Check(content string) (bool, string)
	Severity() int
}

const (
	SeverityLow      = 10
	SeverityMedium   = 30
	SeverityHigh     = 50
	SeverityCritical = 80
)
