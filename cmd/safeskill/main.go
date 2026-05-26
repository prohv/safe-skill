package main

import (
	"flag"
	"fmt"
	"os"

	"safeskill/internal/engine"
	"safeskill/internal/report"
	"safeskill/internal/rules"
	"safeskill/internal/scanner"
)

func main() {
	workers := flag.Int("workers", 4, "number of concurrent workers")
	output := flag.String("output", "", "write report to file")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "usage: safeskill scan <path>\n")
		os.Exit(1)
	}
	if flag.Arg(0) != "scan" {
		fmt.Fprintf(os.Stderr, "usage: safeskill scan <path>\n")
		os.Exit(1)
	}
	if flag.NArg() < 2 {
		fmt.Fprintf(os.Stderr, "usage: safeskill scan <path>\n")
		os.Exit(1)
	}
	path := flag.Arg(1)

	files, err := scanner.Walk(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	pool := scanner.NewPool(*workers, rules.BuiltinRules())
	results := pool.Run(files)
	signals, score := scanner.Aggregate(results)
	status := engine.Classify(score)

	r := report.New(signals, score, status)
	j, err := r.JSON()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(j)

	if *output != "" {
		os.WriteFile(*output, []byte(j+"\n"), 0644)
	}
}
