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

func TestAPIIntegration(t *testing.T) {
	srv := New(Config{ReportsDir: t.TempDir(), Workers: 2})
	ts := httptest.NewServer(srv)
	defer ts.Close()

	t.Run("scan safe package", func(t *testing.T) {
		body := strings.NewReader(`{"path":"../../testdata/safe-pkg"}`)
		resp, err := http.Post(ts.URL+"/scan", "application/json", body)
		if err != nil {
			t.Fatalf("POST error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}

		var rpt report.Report
		if err := json.NewDecoder(resp.Body).Decode(&rpt); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if rpt.Risk != 0 {
			t.Errorf("risk = %d, want 0", rpt.Risk)
		}
		if rpt.Status != "SAFE" {
			t.Errorf("status = %q, want SAFE", rpt.Status)
		}
	})

	t.Run("scan suspicious package", func(t *testing.T) {
		body := strings.NewReader(`{"path":"../../testdata/suspicious-pkg"}`)
		resp, err := http.Post(ts.URL+"/scan", "application/json", body)
		if err != nil {
			t.Fatalf("POST error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}

		var rpt report.Report
		if err := json.NewDecoder(resp.Body).Decode(&rpt); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if rpt.Risk < 30 {
			t.Errorf("risk = %d, want >= 30", rpt.Risk)
		}
		if rpt.Status != "BLOCKED" {
			t.Errorf("status = %q, want BLOCKED", rpt.Status)
		}
		if len(rpt.Signals) == 0 {
			t.Errorf("expected signals, got none")
		}
	})

	t.Run("fetch report after scan", func(t *testing.T) {
		body := strings.NewReader(`{"path":"../../testdata/safe-pkg"}`)
		resp, err := http.Post(ts.URL+"/scan-install", "application/json", body)
		if err != nil {
			t.Fatalf("POST error: %v", err)
		}
		defer resp.Body.Close()

		var installResp struct {
			ReportID string `json:"report_id"`
			Action   string `json:"action"`
			Risk     int    `json:"risk"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&installResp); err != nil {
			t.Fatalf("decode: %v", err)
		}

		getResp, err := http.Get(ts.URL + "/report/" + installResp.ReportID)
		if err != nil {
			t.Fatalf("GET error: %v", err)
		}
		defer getResp.Body.Close()

		if getResp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", getResp.StatusCode)
		}

		var rpt report.Report
		if err := json.NewDecoder(getResp.Body).Decode(&rpt); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if rpt.ID != installResp.ReportID {
			t.Errorf("id = %q, want %q", rpt.ID, installResp.ReportID)
		}
	})

	t.Run("report not found", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/report/nonexistent")
		if err != nil {
			t.Fatalf("GET error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
	})

	t.Run("scan empty directory", func(t *testing.T) {
		emptyDir := t.TempDir()
		bodyBytes, _ := json.Marshal(map[string]string{"path": emptyDir})
		resp, err := http.Post(ts.URL+"/scan", "application/json", strings.NewReader(string(bodyBytes)))
		if err != nil {
			t.Fatalf("POST error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200 (empty dir scans as SAFE)", resp.StatusCode)
		}

		var rpt report.Report
		if err := json.NewDecoder(resp.Body).Decode(&rpt); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if rpt.Status != "SAFE" {
			t.Errorf("status = %q, want SAFE", rpt.Status)
		}
	})
}

func TestAPIIntegration_DiskPersistence(t *testing.T) {
	reportsDir := t.TempDir()
	srv := New(Config{ReportsDir: reportsDir, Workers: 2})
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := strings.NewReader(`{"path":"../../testdata/safe-pkg"}`)
	resp, err := http.Post(ts.URL+"/scan", "application/json", body)
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	defer resp.Body.Close()

	var rpt report.Report
	json.NewDecoder(resp.Body).Decode(&rpt)

	entries, err := os.ReadDir(reportsDir)
	if err != nil {
		t.Fatalf("read reports dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("reports dir has %d entries, want 1", len(entries))
	}
	if entries[0].Name() != rpt.ID+".json" {
		t.Errorf("file = %q, want %q.json", entries[0].Name(), rpt.ID)
	}

	loaded, err := report.Load(reportsDir, rpt.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Risk != 0 {
		t.Errorf("loaded risk = %d, want 0", loaded.Risk)
	}

	io.Copy(io.Discard, resp.Body)
}
