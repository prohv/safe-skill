package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"time"

	"safeskill/internal/cache"
	"safeskill/internal/engine"
	"safeskill/internal/report"
)

type Config struct {
	Port      int
	Upstream  string
	Workers   int
	Threshold int
	CacheTTL  time.Duration
}

type Server struct {
	cfg         Config
	srv         *http.Server
	proxy       *httputil.ReverseProxy
	upstreamURL *url.URL
	cc          *cache.Cache
}

func New(cfg Config) (*Server, error) {
	u, err := url.Parse(cfg.Upstream)
	if err != nil {
		return nil, fmt.Errorf("invalid upstream URL: %w", err)
	}

	rp := httputil.NewSingleHostReverseProxy(u)
	rp.ErrorLog = log.New(os.Stderr, "[proxy] ", log.LstdFlags)

	mux := http.NewServeMux()
	s := &Server{
		cfg:         cfg,
		proxy:       rp,
		upstreamURL: u,
		srv: &http.Server{
			Addr:    fmt.Sprintf(":%d", cfg.Port),
			Handler: mux,
		},
	}

	mux.HandleFunc("/", s.handler)
	s.cc = cache.New(".safeskill/cache", cache.Config{TTL: cfg.CacheTTL})
	return s, nil
}

func (s *Server) Start() error {
	s.ListenAndServeAsync()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	log.Println("proxy shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.Shutdown(ctx)
}

func (s *Server) LogListen() {
	log.Printf("proxy listening on :%d, upstream %s", s.cfg.Port, s.cfg.Upstream)
}

func (s *Server) ListenAndServeAsync() {
	go func() {
		s.LogListen()
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("proxy server error: %v", err)
		}
	}()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.srv.Handler.ServeHTTP(w, r)
}

func (s *Server) handler(w http.ResponseWriter, r *http.Request) {
	if isTarballURL(r.URL.Path) {
		s.handleTarball(w, r)
		return
	}
	s.proxy.ServeHTTP(w, r)
}

func (s *Server) handleTarball(w http.ResponseWriter, r *http.Request) {
	upstreamURL := *r.URL
	upstreamURL.Scheme = s.upstreamURL.Scheme
	upstreamURL.Host = s.upstreamURL.Host

	req, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL.String(), r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusInternalServerError)
		return
	}

	for _, key := range []string{"Accept", "Accept-Encoding", "User-Agent", "Authorization"} {
		if v := r.Header.Get(key); v != "" {
			req.Header.Set(key, v)
		}
	}

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK || !isTarballContent(resp) {
		writeAllowResponse(w, resp.StatusCode, resp.Header, resp.Body)
		return
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTotalExtract*2))
	if err != nil {
		http.Error(w, "read error", http.StatusInternalServerError)
		return
	}

	var result *ScanResult

	if s.cc != nil && s.cfg.CacheTTL > 0 {
		h := cache.Hash(body)
		if rpt, ok := s.cc.Check(h); ok {
			result = &ScanResult{
				Signals: rpt.Signals,
				Score:   rpt.Risk,
				Status:  rpt.Status,
				Report:  rpt,
			}
		}
	}

	if result == nil {
		result, err = ScanTarballInMemory(body, s.cfg.Workers)
		if err != nil {
			http.Error(w, "scan error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if s.cc != nil && s.cfg.CacheTTL > 0 {
			if err := s.cc.Store(cache.Hash(body), result.Report); err != nil {
				log.Printf("proxy: cache store: %v", err)
			}
		}

		if err := report.Save(".safeskill/reports", result.Report); err != nil {
			log.Printf("proxy: save report: %v", err)
		}
	}

	pkgName := packageNameFromURL(r.URL.Path)

	blocked := result.Status == engine.StatusBlocked
	if s.cfg.Threshold > 0 {
		blocked = result.Score >= s.cfg.Threshold
	}

	if blocked {
		LogIntercept(pkgName, "BLOCKED", result.Score, len(result.Signals))
		writeBlockResponse(w, result, "")
		return
	}

	LogIntercept(pkgName, result.Status, result.Score, len(result.Signals))
	writeAllowResponse(w, resp.StatusCode, resp.Header, bytes.NewReader(body))
}

