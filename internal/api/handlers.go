package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"safeskill/internal/proxy"
	"safeskill/internal/report"
)

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Path == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}

	result, err := proxy.RunScan(req.Path, s.cfg.Workers)
	if err != nil {
		http.Error(w, fmt.Sprintf("scan error: %v", err), http.StatusInternalServerError)
		return
	}

	if err := report.Save(s.cfg.ReportsDir, result.Report); err != nil {
		log.Printf("api: save report: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result.Report)
}

func (s *Server) handleScanInstall(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Path == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}

	result, err := proxy.RunScan(req.Path, s.cfg.Workers)
	if err != nil {
		http.Error(w, fmt.Sprintf("scan error: %v", err), http.StatusInternalServerError)
		return
	}

	if err := report.Save(s.cfg.ReportsDir, result.Report); err != nil {
		log.Printf("api: save report: %v", err)
	}

	resp := map[string]interface{}{
		"report_id": result.Report.ID,
		"action":    result.Status,
		"risk":      result.Score,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "report id required", http.StatusBadRequest)
		return
	}

	rpt, err := report.Load(s.cfg.ReportsDir, id)
	if err != nil {
		http.Error(w, "report not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rpt)
}
