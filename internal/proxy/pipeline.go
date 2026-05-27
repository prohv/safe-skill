package proxy

import (
	"safeskill/internal/engine"
	"safeskill/internal/report"
	"safeskill/internal/rules"
	"safeskill/internal/scanner"
	"safeskill/internal/types"
)

type ScanResult struct {
	Signals []types.Signal
	Score   int
	Status  string
	Report  *report.Report
}

func RunScan(dir string, workers int) (*ScanResult, error) {
	files, err := scanner.Walk(dir)
	if err != nil {
		return nil, err
	}

	pool := scanner.NewPool(workers, rules.BuiltinRules())
	results := pool.Run(files)
	signals, score := scanner.Aggregate(results)
	score = engine.ApplyBoosts(signals, score)
	status := engine.Classify(score)
	r := report.New(signals, score, status)

	return &ScanResult{
		Signals: signals,
		Score:   score,
		Status:  status,
		Report:  r,
	}, nil
}
