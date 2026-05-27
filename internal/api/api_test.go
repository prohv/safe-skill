package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"safeskill/internal/report"
)

func TestHandleScan_SafePkg(t *testing.T) {
	srv := New(Config{ReportsDir: t.TempDir(), Workers: 2})
	body := strings.NewReader(`{"path":"../../testdata/safe-pkg"}`)
	req := httptest.NewRequest("POST", "/scan", body)
	rec := httptest.NewRecorder()

	srv.handleScan(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	var rpt report.Report
	if err := json.NewDecoder(rec.Body).Decode(&rpt); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if rpt.ID == "" {
		t.Error("report_id is empty")
	}
	if rpt.Risk != 0 {
		t.Errorf("risk = %d, want 0", rpt.Risk)
	}
	if rpt.Status != "SAFE" {
		t.Errorf("status = %q, want SAFE", rpt.Status)
	}
	if len(rpt.Signals) != 0 {
		t.Errorf("signals = %v, want empty", rpt.Signals)
	}

	entries, err := os.ReadDir(srv.cfg.ReportsDir)
	if err != nil {
		t.Fatalf("read reports dir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("reports dir has %d entries, want 1", len(entries))
	}
}

func TestHandleScan_SuspiciousPkg(t *testing.T) {
	srv := New(Config{ReportsDir: t.TempDir(), Workers: 2})
	body := strings.NewReader(`{"path":"../../testdata/suspicious-pkg"}`)
	req := httptest.NewRequest("POST", "/scan", body)
	rec := httptest.NewRecorder()

	srv.handleScan(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	var rpt report.Report
	if err := json.NewDecoder(rec.Body).Decode(&rpt); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if rpt.ID == "" {
		t.Error("report_id is empty")
	}
	if rpt.Risk < 30 {
		t.Errorf("risk = %d, want >= 30", rpt.Risk)
	}
	if rpt.Status != "BLOCKED" {
		t.Errorf("status = %q, want BLOCKED", rpt.Status)
	}
	if len(rpt.Signals) == 0 {
		t.Error("expected signals, got none")
	}
}

func TestHandleScanInstall(t *testing.T) {
	srv := New(Config{ReportsDir: t.TempDir(), Workers: 2})
	body := strings.NewReader(`{"path":"../../testdata/safe-pkg"}`)
	req := httptest.NewRequest("POST", "/scan-install", body)
	rec := httptest.NewRecorder()

	srv.handleScanInstall(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	var resp struct {
		ReportID string `json:"report_id"`
		Action   string `json:"action"`
		Risk     int    `json:"risk"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ReportID == "" {
		t.Error("report_id is empty")
	}
	if resp.Action != "SAFE" {
		t.Errorf("action = %q, want SAFE", resp.Action)
	}
	if resp.Risk != 0 {
		t.Errorf("risk = %d, want 0", resp.Risk)
	}
}

func TestHandleReport_Found(t *testing.T) {
	reportsDir := t.TempDir()
	srv := New(Config{ReportsDir: reportsDir, Workers: 2})

	r := report.New(nil, 0, "SAFE")
	if err := report.Save(reportsDir, r); err != nil {
		t.Fatalf("save report: %v", err)
	}

	req := httptest.NewRequest("GET", "/report/"+r.ID, nil)
	req.SetPathValue("id", r.ID)
	rec := httptest.NewRecorder()

	srv.handleReport(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	var loaded report.Report
	if err := json.NewDecoder(rec.Body).Decode(&loaded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if loaded.ID != r.ID {
		t.Errorf("id = %q, want %q", loaded.ID, r.ID)
	}
}

func TestHandleReport_NotFound(t *testing.T) {
	srv := New(Config{ReportsDir: t.TempDir(), Workers: 2})

	req := httptest.NewRequest("GET", "/report/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	rec := httptest.NewRecorder()

	srv.handleReport(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}

	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "not found") {
		t.Errorf("body = %q, want 'not found'", string(body))
	}
}

func TestHandleScan_InvalidJSON(t *testing.T) {
	srv := New(Config{ReportsDir: t.TempDir(), Workers: 2})
	body := strings.NewReader(`not json`)
	req := httptest.NewRequest("POST", "/scan", body)
	rec := httptest.NewRecorder()

	srv.handleScan(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleScan_MissingPath(t *testing.T) {
	srv := New(Config{ReportsDir: t.TempDir(), Workers: 2})
	body := strings.NewReader(`{"path":""}`)
	req := httptest.NewRequest("POST", "/scan", body)
	rec := httptest.NewRecorder()

	srv.handleScan(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleScanInstall_MissingPath(t *testing.T) {
	srv := New(Config{ReportsDir: t.TempDir(), Workers: 2})
	body := strings.NewReader(`{}`)
	req := httptest.NewRequest("POST", "/scan-install", body)
	rec := httptest.NewRecorder()

	srv.handleScanInstall(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestConfigDefaults(t *testing.T) {
	srv := New(Config{})
	if srv.cfg.Port != 9090 {
		t.Errorf("port = %d, want 9090", srv.cfg.Port)
	}
	if srv.cfg.ReportsDir != ".safeskill/reports" {
		t.Errorf("reports dir = %q, want .safeskill/reports", srv.cfg.ReportsDir)
	}
	if srv.cfg.Workers != 4 {
		t.Errorf("workers = %d, want 4", srv.cfg.Workers)
	}
}
