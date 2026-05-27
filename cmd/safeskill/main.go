package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"time"

	"safeskill/internal/api"
	"safeskill/internal/cli"
	"safeskill/internal/config"
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
	fmt.Fprintf(os.Stderr, "  proxy run     setup + start + teardown on exit\n")
	fmt.Fprintf(os.Stderr, "  proxy wrap    setup + proxy + npm install + teardown\n")
	fmt.Fprintf(os.Stderr, "  proxy setup   configure npm to use the proxy\n")
	fmt.Fprintf(os.Stderr, "  proxy tear    restore npm config to defaults\n")
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
	if cli.IsTerminal() {
		fmt.Printf("%s (Risk: %d)\n", cli.ColorStatus(status), score)
	}
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
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: safeskill proxy start [flags]\n")
		fmt.Fprintf(os.Stderr, "       safeskill proxy run   [flags]  (setup + start + tear)\n")
		fmt.Fprintf(os.Stderr, "       safeskill proxy wrap  [flags] -- <npm cmd>\n")
		fmt.Fprintf(os.Stderr, "       safeskill proxy setup\n")
		fmt.Fprintf(os.Stderr, "       safeskill proxy tear\n")
		os.Exit(1)
	}

	switch args[0] {
	case "start":
		runProxyStart(args[1:])
	case "run":
		runProxyRun(args[1:])
	case "wrap":
		runProxyWrap(args[1:])
	case "setup":
		runProxySetup()
	case "tear":
		runProxyTear()
	default:
		fmt.Fprintf(os.Stderr, "usage: safeskill proxy start [flags]\n")
		fmt.Fprintf(os.Stderr, "       safeskill proxy run   [flags]  (setup + start + tear)\n")
		fmt.Fprintf(os.Stderr, "       safeskill proxy wrap  [flags] -- <npm cmd>\n")
		fmt.Fprintf(os.Stderr, "       safeskill proxy setup\n")
		fmt.Fprintf(os.Stderr, "       safeskill proxy tear\n")
		os.Exit(1)
	}
}

func newProxyServer(args []string) (*proxy.Server, error) {
	fileCfg, _ := config.Load(".safeskill/config.json")

	defaultPort := 8080
	defaultUpstream := "https://registry.npmjs.org"
	defaultThreshold := 0
	defaultWorkers := 4
	defaultCacheTTL := 24 * time.Hour

	if fileCfg != nil {
		if fileCfg.Proxy.Port > 0 {
			defaultPort = fileCfg.Proxy.Port
		}
		if fileCfg.Proxy.Upstream != "" {
			defaultUpstream = fileCfg.Proxy.Upstream
		}
		if fileCfg.Threshold > 0 {
			defaultThreshold = fileCfg.Threshold
		}
		if fileCfg.Workers > 0 {
			defaultWorkers = fileCfg.Workers
		}
		if fileCfg.Cache.TTL != "" {
			defaultCacheTTL = fileCfg.CacheTTL()
		}
	}

	fs := flag.NewFlagSet("proxy", flag.ExitOnError)
	port := fs.Int("port", defaultPort, "proxy listen port")
	upstream := fs.String("upstream", defaultUpstream, "upstream npm registry URL")
	threshold := fs.Int("threshold", defaultThreshold, "override block threshold (0 = use engine default 70)")
	workers := fs.Int("workers", defaultWorkers, "number of scan workers")
	fs.Parse(args)

	return proxy.New(proxy.Config{
		Port:      *port,
		Upstream:  *upstream,
		Workers:   *workers,
		Threshold: *threshold,
		CacheTTL:  defaultCacheTTL,
	})
}

func runProxyStart(args []string) {
	srv, err := newProxyServer(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runProxyRun(args []string) {
	runProxySetup()

	srv, err := newProxyServer(args)
	if err != nil {
		runProxyTear()
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	srv.ListenAndServeAsync()
	log.Println("proxy running in background — Ctrl+C to stop and restore npm config")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	log.Println("stopping proxy...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)

	runProxyTear()
}

func runProxySetup() {
	if err := exec.Command("npm", "config", "set", "registry", "http://registry.npmjs.org/").Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: npm not found or failed\n")
		os.Exit(1)
	}
	exec.Command("npm", "config", "set", "proxy", "http://localhost:8080").Run()
	exec.Command("npm", "config", "set", "https-proxy", "").Run()
	fmt.Println("npm configured: registry=http proxy=localhost:8080 https-proxy=off")
}

func runProxyTear() {
	exec.Command("npm", "config", "delete", "registry").Run()
	exec.Command("npm", "config", "delete", "proxy").Run()
	exec.Command("npm", "config", "delete", "https-proxy").Run()
	fmt.Println("npm config restored to defaults")
}

func splitWrap(args []string) (proxyArgs, cmdArgs []string) {
	for i, a := range args {
		if a == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

func runProxyWrap(args []string) {
	proxyArgs, cmdArgs := splitWrap(args)
	if len(cmdArgs) == 0 {
		fmt.Fprintf(os.Stderr, "usage: safeskill proxy wrap [flags] -- <npm command>\n")
		os.Exit(1)
	}

	runProxySetup()

	srv, err := newProxyServer(proxyArgs)
	if err != nil {
		runProxyTear()
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	srv.ListenAndServeAsync()
	log.Println("proxy ready — running npm, stand by...")

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	exitCode := 0
	if err := cmd.Run(); err != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	log.Println("npm finished, stopping proxy...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)

	runProxyTear()
	os.Exit(exitCode)
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
