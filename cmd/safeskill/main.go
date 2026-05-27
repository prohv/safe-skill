package main

import (
	"flag"
	"fmt"
	"os"

	"safeskill/internal/api"
	"safeskill/internal/engine"
	"safeskill/internal/proxy"
	"safeskill/internal/report"
	"safeskill/internal/rules"
	"safeskill/internal/scanner"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "scan":
		runScan(os.Args[2:])
	case "proxy":
		runProxy(os.Args[2:])
	case "api":
		runAPI(os.Args[2:])
	case "report":
		runReport(os.Args[2:])
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "usage: safeskill <command> [flags]\n")
	fmt.Fprintf(os.Stderr, "commands:\n")
	fmt.Fprintf(os.Stderr, "  scan <path>   scan a package directory\n")
	fmt.Fprintf(os.Stderr, "  proxy start   start the proxy server\n")
	fmt.Fprintf(os.Stderr, "  api start     start the API server\n")
	fmt.Fprintf(os.Stderr, "  report <id>   fetch a scan report\n")
}

func runScan(args []string) {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	workers := fs.Int("workers", 4, "number of concurrent workers")
	output := fs.String("output", "", "write report to file")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "usage: safeskill scan <path>\n")
		os.Exit(1)
	}
	path := fs.Arg(0)

	files, err := scanner.Walk(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	pool := scanner.NewPool(*workers, rules.BuiltinRules())
	results := pool.Run(files)
	signals, score := scanner.Aggregate(results)
	score = engine.ApplyBoosts(signals, score)
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

func runProxy(args []string) {
	if len(args) < 1 || args[0] != "start" {
		fmt.Fprintf(os.Stderr, "usage: safeskill proxy start [flags]\n")
		os.Exit(1)
	}

	fs := flag.NewFlagSet("proxy start", flag.ExitOnError)
	port := fs.Int("port", 8080, "proxy listen port")
	upstream := fs.String("upstream", "https://registry.npmjs.org", "upstream npm registry URL")
	threshold := fs.Int("threshold", 0, "override block threshold (0 = use engine default 70)")
	workers := fs.Int("workers", 4, "number of scan workers")
	fs.Parse(args[1:])

	srv, err := proxy.New(proxy.Config{
		Port:      *port,
		Upstream:  *upstream,
		Workers:   *workers,
		Threshold: *threshold,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runAPI(args []string) {
	if len(args) < 1 || args[0] != "start" {
		fmt.Fprintf(os.Stderr, "usage: safeskill api start [flags]\n")
		os.Exit(1)
	}

	fs := flag.NewFlagSet("api start", flag.ExitOnError)
	port := fs.Int("port", 9090, "api listen port")
	reportsDir := fs.String("reports-dir", ".safeskill/reports", "reports directory")
	workers := fs.Int("workers", 4, "number of scan workers")
	fs.Parse(args[1:])

	srv := api.New(api.Config{
		Port:       *port,
		ReportsDir: *reportsDir,
		Workers:    *workers,
	})

	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runReport(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: safeskill report <id>\n")
		os.Exit(1)
	}

	r, err := report.Load(".safeskill/reports", args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: report not found\n")
		os.Exit(1)
	}

	j, err := r.JSON()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(j)
}
