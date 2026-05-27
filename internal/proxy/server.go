package proxy

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"time"
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

func (s *Server) handler(w http.ResponseWriter, r *http.Request) {
	log.Printf("proxy %s %s", r.Method, r.URL.Path)
	s.proxy.ServeHTTP(w, r)
}
