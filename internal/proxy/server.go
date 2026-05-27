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

	"safeskill/internal/engine"
)

type Config struct {
	Port      int
	Upstream  string
	Workers   int
	Threshold int
}

type Server struct {
	cfg         Config
	srv         *http.Server
	proxy       *httputil.ReverseProxy
	upstreamURL *url.URL
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
	return s, nil
}

func (s *Server) Start() error {
	go func() {
		log.Printf("proxy listening on :%d, upstream %s", s.cfg.Port, s.cfg.Upstream)
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("proxy server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	log.Println("proxy shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
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

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK || !isTarballContent(resp) {
		writeAllowResponse(w, resp.StatusCode, resp.Header, resp.Body)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusInternalServerError)
		return
	}

	tmpDir, err := os.MkdirTemp("", "safeskill-extract-")
	if err != nil {
		http.Error(w, "temp dir error", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmpDir)

	_, err = ExtractTarball(bytes.NewReader(body), tmpDir)
	if err != nil {
		http.Error(w, "extract error", http.StatusInternalServerError)
		return
	}

	result, err := RunScan(tmpDir, s.cfg.Workers)
	if err != nil {
		http.Error(w, "scan error", http.StatusInternalServerError)
		return
	}

	blocked := result.Status == engine.StatusBlocked
	if s.cfg.Threshold > 0 {
		blocked = result.Score >= s.cfg.Threshold
	}

	if blocked {
		writeBlockResponse(w, result, "")
		return
	}

	writeAllowResponse(w, resp.StatusCode, resp.Header, bytes.NewReader(body))
}
