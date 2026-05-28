package proxy

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"sync"

	"safeskill/internal/engine"
	"safeskill/internal/report"
	"safeskill/internal/rules"
	"safeskill/internal/scanner"
	"safeskill/internal/types"
)

type ExtractedFile struct {
	Name    string
	Content []byte
}

func ScanTarballInMemory(body []byte, workers int) (*ScanResult, error) {
	gz, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var files []ExtractedFile
	var totalRead int64

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		if hdr.Size > maxExtractSize {
			continue
		}
		if totalRead+hdr.Size > maxTotalExtract {
			continue
		}

		content, err := io.ReadAll(io.LimitReader(tr, hdr.Size))
		if err != nil {
			continue
		}

		files = append(files, ExtractedFile{Name: hdr.Name, Content: content})
		totalRead += int64(len(content))
	}

	results := scanFilesInParallel(files, workers, rules.BuiltinRules())

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

func scanFilesInParallel(files []ExtractedFile, workers int, rules []types.Rule) []scanner.Result {
	jobs := make(chan ExtractedFile, len(files))
	results := make(chan scanner.Result, len(files))

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range jobs {
				var signals []types.Signal
				for _, rule := range rules {
					if matched, msg := rule.Check(string(f.Content)); matched {
						signals = append(signals, types.Signal{
							Rule:     rule.Name(),
							Message:  msg,
							Severity: rule.Severity(),
						})
					}
				}
				if len(signals) > 0 {
					results <- scanner.Result{Path: f.Name, Signals: signals}
				}
			}
		}()
	}

	for _, f := range files {
		jobs <- f
	}
	close(jobs)

	wg.Wait()
	close(results)

	var out []scanner.Result
	for r := range results {
		out = append(out, r)
	}
	return out
}
