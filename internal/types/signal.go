package types

type Signal struct {
	Rule     string `json:"rule"`
	Message  string `json:"message"`
	Severity int    `json:"severity"`
}
