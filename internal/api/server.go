package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
)

type Config struct {
	Port       int
	ReportsDir string
	Workers    int
}

type Server struct {
	cfg Config
	srv *http.Server
}

func New(cfg Config) *Server {
	if cfg.Port == 0 {
		cfg.Port = 9090
	}
	if cfg.ReportsDir == "" {
		cfg.ReportsDir = ".safeskill/reports"
	}
	if cfg.Workers == 0 {
		cfg.Workers = 4
	}

	mux := http.NewServeMux()
	s := &Server{
		cfg: cfg,
		srv: &http.Server{
			Addr:    fmt.Sprintf(":%d", cfg.Port),
			Handler: mux,
		},
	}

	mux.HandleFunc("POST /scan", s.handleScan)
	mux.HandleFunc("POST /scan-install", s.handleScanInstall)
	mux.HandleFunc("GET /report/{id}", s.handleReport)
	return s
}

func (s *Server) Start() error {
	go func() {
		log.Printf("api listening on :%d", s.cfg.Port)
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("api server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	log.Println("api shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.srv.Shutdown(ctx)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.srv.Handler.ServeHTTP(w, r)
}
