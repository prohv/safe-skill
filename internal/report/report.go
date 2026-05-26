package report

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"

	"safeskill/internal/types"
)

type Report struct {
	ID      string         `json:"report_id"`
	Risk    int            `json:"risk"`
	Status  string         `json:"status"`
	Signals []types.Signal `json:"signals"`
	Summary string         `json:"summary"`
}

func New(signals []types.Signal, score int, status string) *Report {
	return &Report{
		ID:      genID(),
		Risk:    score,
		Status:  status,
		Signals: signals,
		Summary: summarize(signals),
	}
}

func (r *Report) JSON() (string, error) {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func genID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func summarize(signals []types.Signal) string {
	if len(signals) == 0 {
		return "No threats detected"
	}
	return signals[0].Message
}
