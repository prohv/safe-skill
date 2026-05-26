package scanner

import (
	"os"
	"sync"

	"safeskill/internal/types"
)

type Job struct {
	Path string
}

type Result struct {
	Path    string
	Signals []types.Signal
}

type Pool struct {
	workers int
	rules   []types.Rule
}

func NewPool(workers int, rules []types.Rule) *Pool {
	if workers < 1 {
		workers = 4
	}
	return &Pool{workers: workers, rules: rules}
}

func (p *Pool) Run(files []string) []Result {
	jobs := make(chan Job, len(files))
	results := make(chan Result, len(files))

	var wg sync.WaitGroup
	for i := 0; i < p.workers; i++ {
		wg.Add(1)
		go p.worker(jobs, results, &wg)
	}

	for _, f := range files {
		jobs <- Job{Path: f}
	}
	close(jobs)

	wg.Wait()
	close(results)

	var out []Result
	for r := range results {
		out = append(out, r)
	}
	return out
}

func (p *Pool) worker(jobs <-chan Job, results chan<- Result, wg *sync.WaitGroup) {
	defer wg.Done()
	for job := range jobs {
		content, err := os.ReadFile(job.Path)
		if err != nil {
			continue
		}
		var signals []types.Signal
		for _, rule := range p.rules {
			if matched, msg := rule.Check(string(content)); matched {
				signals = append(signals, types.Signal{
					Rule:     rule.Name(),
					Message:  msg,
					Severity: rule.Severity(),
				})
			}
		}
		if len(signals) > 0 {
			results <- Result{Path: job.Path, Signals: signals}
		}
	}
}
