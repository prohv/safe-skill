package proxy

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func buildTestTarball(t *testing.T, dir string) []byte {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil || rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if d.IsDir() {
			tw.WriteHeader(&tar.Header{
				Name:     rel + "/",
				Mode:     0755,
				Typeflag: tar.TypeDir,
			})
			return nil
		}
		tw.WriteHeader(&tar.Header{
			Name: rel,
			Size: info.Size(),
			Mode: 0644,
		})
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		tw.Write(data)
		return nil
	})
	if err != nil {
		t.Fatalf("building tarball from %s: %v", dir, err)
	}

	tw.Close()
	gw.Close()
	return buf.Bytes()
}

func TestProxyIntegration(t *testing.T) {
	safeTar := buildTestTarball(t, filepath.Join("..", "..", "testdata", "safe-pkg"))
	blockedTar := buildTestTarball(t, filepath.Join("..", "..", "testdata", "suspicious-pkg"))

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		if strings.Contains(r.URL.Path, "safe") {
			w.Write(safeTar)
		} else {
			w.Write(blockedTar)
		}
	}))
	defer upstream.Close()

	srv, err := New(Config{
		Upstream: upstream.URL,
		Workers:  2,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	proxyServer := httptest.NewServer(srv)
	defer proxyServer.Close()

	t.Run("blocks suspicious package", func(t *testing.T) {
		resp, err := http.Get(proxyServer.URL + "/suspicious/pkg/-/pkg-1.0.0.tgz")
		if err != nil {
			t.Fatalf("GET error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403, got %d", resp.StatusCode)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("JSON decode error: %v", err)
		}

		if body["status"] != "BLOCKED" {
			t.Errorf("status = %v, want BLOCKED", body["status"])
		}
		signals, ok := body["signals"].([]interface{})
		if !ok || len(signals) == 0 {
			t.Errorf("expected non-empty signals, got %v", body["signals"])
		}
		risk, ok := body["risk"].(float64)
		if !ok || risk < 30 {
			t.Errorf("expected risk >= 30, got %v", risk)
		}
		if body["report_id"] == "" {
			t.Errorf("expected non-empty report_id")
		}
	})

	t.Run("allows safe package", func(t *testing.T) {
		resp, err := http.Get(proxyServer.URL + "/safe/pkg/-/safe-1.0.0.tgz")
		if err != nil {
			t.Fatalf("GET error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read error: %v", err)
		}
		if !bytes.Equal(body, safeTar) {
			t.Errorf("response body does not match original tarball (len got=%d want=%d)", len(body), len(safeTar))
		}
	})

	t.Run("passes through non-tarball", func(t *testing.T) {
		resp, err := http.Get(proxyServer.URL + "/some-package/metadata")
		if err != nil {
			t.Fatalf("GET error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200 for passthrough, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read error: %v", err)
		}
		if len(body) != len(safeTar) && len(body) != len(blockedTar) {
			t.Errorf("unexpected body length %d (passthrough should proxy upstream)", len(body))
		}
	})
}

func TestProxyIntegration_UpstreamError(t *testing.T) {
	t.Run("upstream returns 500 for tarball URL", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("upstream error"))
		}))
		defer upstream.Close()

		srv, err := New(Config{
			Upstream: upstream.URL,
			Workers:  2,
		})
		if err != nil {
			t.Fatalf("New() error: %v", err)
		}

		proxyServer := httptest.NewServer(srv)
		defer proxyServer.Close()

		resp, err := http.Get(proxyServer.URL + "/pkg/-/pkg-1.0.0.tgz")
		if err != nil {
			t.Fatalf("GET error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", resp.StatusCode)
		}
	})

	t.Run("wrong content type despite tgz URL", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(`{"name":"pkg"}`))
		}))
		defer upstream.Close()

		srv, err := New(Config{
			Upstream: upstream.URL,
			Workers:  2,
		})
		if err != nil {
			t.Fatalf("New() error: %v", err)
		}

		proxyServer := httptest.NewServer(srv)
		defer proxyServer.Close()

		resp, err := http.Get(proxyServer.URL + "/pkg/-/pkg-1.0.0.tgz")
		if err != nil {
			t.Fatalf("GET error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200 (passthrough for non-tarball content), got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if string(body) != `{"name":"pkg"}` {
			t.Errorf("body = %q, want upstream response", string(body))
		}
	})
}
