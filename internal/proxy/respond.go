package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"safeskill/internal/types"
)

func writeAllowResponse(w http.ResponseWriter, statusCode int, headers http.Header, body io.Reader) {
	for k, vs := range headers {
		lower := strings.ToLower(k)
		if lower == "connection" || lower == "keep-alive" || lower == "transfer-encoding" {
			continue
		}
		if strings.HasPrefix(lower, "proxy-") {
			continue
		}
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(statusCode)
	io.Copy(w, body)
}

func writeBlockResponse(w http.ResponseWriter, result *ScanResult, reason string) {
	if reason == "" && result != nil {
		reason = result.Report.Summary
	}
	if reason == "" {
		reason = "blocked by security policy"
	}

	var signals []types.Signal
	risk := 0
	reportID := ""
	if result != nil {
		signals = result.Signals
		risk = result.Score
		reportID = result.Report.ID
	}

	body, _ := json.Marshal(map[string]interface{}{
		"reason":    reason,
		"status":    "BLOCKED",
		"risk":      risk,
		"signals":   signals,
		"report_id": reportID,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	w.Write(body)
}
