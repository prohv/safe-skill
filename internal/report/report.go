package report

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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

func Save(dir string, r *Report) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("save: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, r.ID+".json"), data, 0644)
}

func Load(dir string, id string) (*Report, error) {
	data, err := os.ReadFile(filepath.Join(dir, id+".json"))
	if err != nil {
		return nil, fmt.Errorf("load: %w", err)
	}
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("load: %w", err)
	}
	return &r, nil
}

func genID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
}

func summarize(signals []types.Signal) string {
	if len(signals) == 0 {
		return "No threats detected"
	}
	return signals[0].Message
}
