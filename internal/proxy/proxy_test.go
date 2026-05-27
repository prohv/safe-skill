package proxy

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"safeskill/internal/report"
	"safeskill/internal/types"
)

type tarEntry struct {
	name     string
	body     string
	mode     int64
	typeflag byte
	link     string
}

func createTestTar(entries []tarEntry) []byte {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for _, e := range entries {
		tw.WriteHeader(&tar.Header{
			Name:     e.name,
			Size:     int64(len(e.body)),
			Mode:     e.mode,
			Typeflag: e.typeflag,
			Linkname: e.link,
		})
		tw.Write([]byte(e.body))
	}
	tw.Close()
	gw.Close()
	return buf.Bytes()
}

func TestIsTarballURL(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"scoped tgz", "/@scope/pkg/-/pkg-1.0.0.tgz", true},
		{"unscoped tar.gz", "/pkg/-/pkg-1.0.0.tar.gz", true},
		{"case insensitive", "/pkg/-/pkg-1.0.0.TGZ", true},
		{"metadata path", "/@scope/pkg", false},
		{"plain path", "/pkg", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTarballURL(tt.path); got != tt.want {
				t.Errorf("isTarballURL(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsTarballContent(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		want        bool
	}{
		{"gzip", "application/gzip", true},
		{"octet-stream", "application/octet-stream", true},
		{"gzip with charset", "application/gzip; charset=utf-8", true},
		{"x-tar", "application/x-tar", true},
		{"x-compressed", "application/x-compressed", true},
		{"gzip-compressed", "application/gzip-compressed", true},
		{"text/plain", "text/plain", false},
		{"empty header", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{Header: http.Header{}}
			resp.Header.Set("Content-Type", tt.contentType)
			if got := isTarballContent(resp); got != tt.want {
				t.Errorf("isTarballContent(%q) = %v, want %v", tt.contentType, got, tt.want)
			}
		})
	}
}

func TestPackageNameFromURL(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"scoped", "/@scope/package/-/package-1.0.0.tgz", "@scope/package"},
		{"unscoped", "/package/-/package-1.0.0.tgz", "package"},
		{"deep path", "/a/b/c/d/e.tgz", "a"},
		{"root only", "/", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := packageNameFromURL(tt.path); got != tt.want {
				t.Errorf("packageNameFromURL(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestExtractTarball(t *testing.T) {
	t.Run("valid single file", func(t *testing.T) {
		data := createTestTar([]tarEntry{
			{name: "index.js", body: "console.log(1)", mode: 0644, typeflag: tar.TypeReg},
		})
		dest := t.TempDir()
		files, err := ExtractTarball(bytes.NewReader(data), dest)
		if err != nil {
			t.Fatalf("ExtractTarball() error: %v", err)
		}
		if len(files) != 1 {
			t.Fatalf("got %d files, want 1", len(files))
		}
		b, _ := os.ReadFile(files[0])
		if string(b) != "console.log(1)" {
			t.Errorf("content = %q, want %q", string(b), "console.log(1)")
		}
	})

	t.Run("multiple files and dirs", func(t *testing.T) {
		data := createTestTar([]tarEntry{
			{name: "src/", body: "", mode: 0755, typeflag: tar.TypeDir},
			{name: "src/util.js", body: "util", mode: 0644, typeflag: tar.TypeReg},
			{name: "package.json", body: `{"name":"test"}`, mode: 0644, typeflag: tar.TypeReg},
		})
		dest := t.TempDir()
		files, err := ExtractTarball(bytes.NewReader(data), dest)
		if err != nil {
			t.Fatalf("ExtractTarball() error: %v", err)
		}
		if len(files) != 2 {
			t.Errorf("got %d files, want 2", len(files))
		}
	})

	t.Run("zip slip blocked", func(t *testing.T) {
		data := createTestTar([]tarEntry{
			{name: "../../../etc/passwd", body: "root:x:0:0", mode: 0644, typeflag: tar.TypeReg},
		})
		dest := t.TempDir()
		files, err := ExtractTarball(bytes.NewReader(data), dest)
		if err != nil {
			t.Fatalf("ExtractTarball() error: %v", err)
		}
		if len(files) != 0 {
			t.Errorf("expected 0 files for zip-slip, got %d", len(files))
		}
		_, err = os.Stat(filepath.Join(dest, "..", "..", "passwd"))
		if !os.IsNotExist(err) {
			t.Errorf("zip-slip file was written to disk")
		}
	})

	t.Run("oversized file skipped", func(t *testing.T) {
		bigBody := strings.Repeat("A", maxExtractSize+1)
		data := createTestTar([]tarEntry{
			{name: "big.bin", body: bigBody, mode: 0644, typeflag: tar.TypeReg},
		})
		dest := t.TempDir()
		files, err := ExtractTarball(bytes.NewReader(data), dest)
		if err != nil {
			t.Fatalf("ExtractTarball() error: %v", err)
		}
		if len(files) != 0 {
			t.Errorf("expected 0 files, got %d", len(files))
		}
	})

	t.Run("nested depth limited", func(t *testing.T) {
		deepPath := strings.Repeat("a/", maxExtractDepth+1) + "deep.js"
		data := createTestTar([]tarEntry{
			{name: deepPath, body: "deep", mode: 0644, typeflag: tar.TypeReg},
		})
		dest := t.TempDir()
		files, err := ExtractTarball(bytes.NewReader(data), dest)
		if err != nil {
			t.Fatalf("ExtractTarball() error: %v", err)
		}
		if len(files) != 0 {
			t.Errorf("expected 0 files for too-deep path, got %d", len(files))
		}
	})

	t.Run("empty tar", func(t *testing.T) {
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		gw.Close()
		dest := t.TempDir()
		files, err := ExtractTarball(bytes.NewReader(buf.Bytes()), dest)
		if err != nil {
			t.Fatalf("ExtractTarball() error: %v", err)
		}
		if len(files) != 0 {
			t.Errorf("expected 0 files, got %d", len(files))
		}
	})

	t.Run("invalid gzip", func(t *testing.T) {
		dest := t.TempDir()
		_, err := ExtractTarball(bytes.NewReader([]byte("not-gzip-data")), dest)
		if err == nil {
			t.Fatal("expected error for invalid gzip")
		}
		if !strings.Contains(err.Error(), "gzip") {
			t.Errorf("error = %q, want substring 'gzip'", err.Error())
		}
	})

	t.Run("symlink in bounds", func(t *testing.T) {
		check := t.TempDir()
		if err := os.Symlink("target", filepath.Join(check, "test-link")); err != nil {
			t.Skipf("symlinks not supported on this system: %v", err)
		}

		data := createTestTar([]tarEntry{
			{name: "real.js", body: "real", mode: 0644, typeflag: tar.TypeReg},
			{name: "link.js", body: "", mode: 0777, typeflag: tar.TypeSymlink, link: "real.js"},
		})
		dest := t.TempDir()
		files, err := ExtractTarball(bytes.NewReader(data), dest)
		if err != nil {
			t.Fatalf("ExtractTarball() error: %v", err)
		}
		if len(files) != 1 {
			t.Errorf("got %d files, want 1 (symlink not counted)", len(files))
		}
		linkPath := filepath.Join(dest, "link.js")
		info, err := os.Lstat(linkPath)
		if err != nil {
			t.Fatalf("symlink not created: %v", err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("link.js is not a symlink")
		}
	})
}

func TestWriteAllowResponse(t *testing.T) {
	t.Run("normal headers", func(t *testing.T) {
		rec := httptest.NewRecorder()
		headers := http.Header{}
		headers.Set("Content-Type", "text/plain")
		headers.Set("Cache-Control", "public")

		writeAllowResponse(rec, 200, headers, strings.NewReader("hello"))

		resp := rec.Result()
		if resp.StatusCode != 200 {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		if resp.Header.Get("Content-Type") != "text/plain" {
			t.Errorf("Content-Type = %q, want text/plain", resp.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(resp.Body)
		if string(body) != "hello" {
			t.Errorf("body = %q, want %q", string(body), "hello")
		}
	})

	t.Run("hop-by-hop stripped", func(t *testing.T) {
		rec := httptest.NewRecorder()
		headers := http.Header{}
		headers.Set("Connection", "keep-alive")
		headers.Set("Keep-Alive", "timeout=5")
		headers.Set("Transfer-Encoding", "chunked")
		headers.Set("Content-Type", "text/plain")

		writeAllowResponse(rec, 200, headers, strings.NewReader("ok"))

		resp := rec.Result()
		if resp.Header.Get("Connection") != "" {
			t.Errorf("Connection header should be stripped, got %q", resp.Header.Get("Connection"))
		}
		if resp.Header.Get("Keep-Alive") != "" {
			t.Errorf("Keep-Alive header should be stripped, got %q", resp.Header.Get("Keep-Alive"))
		}
		if resp.Header.Get("Content-Type") != "text/plain" {
			t.Errorf("Content-Type = %q, want text/plain", resp.Header.Get("Content-Type"))
		}
	})

	t.Run("proxy headers stripped", func(t *testing.T) {
		rec := httptest.NewRecorder()
		headers := http.Header{}
		headers.Set("Proxy-Authenticate", "Basic")
		headers.Set("Proxy-Authorization", "token")

		writeAllowResponse(rec, 200, headers, strings.NewReader(""))

		resp := rec.Result()
		if resp.Header.Get("Proxy-Authenticate") != "" {
			t.Errorf("Proxy-Authenticate should be stripped")
		}
		if resp.Header.Get("Proxy-Authorization") != "" {
			t.Errorf("Proxy-Authorization should be stripped")
		}
	})
}

func TestWriteBlockResponse(t *testing.T) {
	t.Run("with result", func(t *testing.T) {
		rec := httptest.NewRecorder()
		result := &ScanResult{
			Signals: []types.Signal{
				{Rule: "ShellExec", Message: "executes shell", Severity: 50},
			},
			Score:  80,
			Status: "BLOCKED",
			Report: report.New(
				[]types.Signal{{Rule: "ShellExec", Message: "executes shell", Severity: 50}},
				80, "BLOCKED",
			),
		}

		writeBlockResponse(rec, result, "")

		resp := rec.Result()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("status = %d, want 403", resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}

		var body map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&body)

		if body["status"] != "BLOCKED" {
			t.Errorf("status = %v, want BLOCKED", body["status"])
		}
		if body["risk"].(float64) != 80 {
			t.Errorf("risk = %v, want 80", body["risk"])
		}
		if body["reason"] != "executes shell" {
			t.Errorf("reason = %v, want 'executes shell'", body["reason"])
		}
		signals := body["signals"].([]interface{})
		if len(signals) != 1 {
			t.Errorf("signals len = %d, want 1", len(signals))
		}
		if body["report_id"] == "" {
			t.Errorf("report_id should not be empty")
		}
	})

	t.Run("nil result with reason", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writeBlockResponse(rec, nil, "custom reason")

		var body map[string]interface{}
		resp := rec.Result()
		json.NewDecoder(resp.Body).Decode(&body)

		if body["reason"] != "custom reason" {
			t.Errorf("reason = %v, want 'custom reason'", body["reason"])
		}
		if risk := body["risk"].(float64); risk != 0 {
			t.Errorf("risk = %v, want 0", risk)
		}
	})

	t.Run("nil result no reason", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writeBlockResponse(rec, nil, "")

		var body map[string]interface{}
		resp := rec.Result()
		json.NewDecoder(resp.Body).Decode(&body)

		if body["reason"] != "blocked by security policy" {
			t.Errorf("reason = %v, want default", body["reason"])
		}
	})
}

func TestRunScan(t *testing.T) {
	t.Run("safe package", func(t *testing.T) {
		root := filepath.Join("..", "..", "testdata", "safe-pkg")
		result, err := RunScan(root, 2)
		if err != nil {
			t.Fatalf("RunScan() error: %v", err)
		}
		if result.Score >= 30 {
			t.Errorf("score = %d, want < 30", result.Score)
		}
	})

	t.Run("suspicious package", func(t *testing.T) {
		root := filepath.Join("..", "..", "testdata", "suspicious-pkg")
		result, err := RunScan(root, 2)
		if err != nil {
			t.Fatalf("RunScan() error: %v", err)
		}
		if result.Score < 30 {
			t.Errorf("score = %d, want >= 30", result.Score)
		}
		if len(result.Signals) == 0 {
			t.Errorf("expected signals, got none")
		}
	})
}

func TestIsSubPath(t *testing.T) {
	tests := []struct {
		name   string
		parent string
		child  string
		want   bool
	}{
		{"same dir", "/a/b", "/a/b", true},
		{"child dir", "/a", "/a/b/c", true},
		{"traversal", "/a", "/a/../../etc", false},
		{"unrelated path", "/a", "/b", false},
		{"subdir traversal", "/a/b", "/a/b/c/../../../../etc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSubPath(tt.parent, tt.child); got != tt.want {
				t.Errorf("isSubPath(%q, %q) = %v, want %v", tt.parent, tt.child, got, tt.want)
			}
		})
	}
}

func TestRelDepth(t *testing.T) {
	tests := []struct {
		name   string
		parent string
		child  string
		want   int
	}{
		{"same", "/a", "/a", 0},
		{"one level", "/a", "/a/b", 1},
		{"two levels", "/a", "/a/b/c", 2},
		{"unrelated", "/a", "/b", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := relDepth(tt.parent, tt.child); got != tt.want {
				t.Errorf("relDepth(%q, %q) = %d, want %d", tt.parent, tt.child, got, tt.want)
			}
		})
	}
}

func TestLogIntercept(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	LogIntercept("@scope/pkg", "SAFE", 12, 3)

	output := buf.String()
	if !strings.Contains(output, "@scope/pkg") {
		t.Errorf("log missing package name: %s", output)
	}
	if !strings.Contains(output, "SAFE") {
		t.Errorf("log missing status: %s", output)
	}
	if !strings.Contains(output, "risk=12") {
		t.Errorf("log missing risk: %s", output)
	}
	if !strings.Contains(output, "files=3") {
		t.Errorf("log missing file count: %s", output)
	}
}

func TestExtractTarball_TarBomb(t *testing.T) {
	entries := make([]tarEntry, 1000)
	for i := range entries {
		entries[i] = tarEntry{
			name: fmt.Sprintf("f%d", i),
			body: "x",
			mode: 0644,
			typeflag: tar.TypeReg,
		}
	}
	data := createTestTar(entries)
	dest := t.TempDir()
	files, err := ExtractTarball(bytes.NewReader(data), dest)
	if err != nil {
		t.Fatalf("ExtractTarball() error: %v", err)
	}
	if len(files) != 1000 {
		t.Errorf("got %d files, want 1000", len(files))
	}
}

func TestExtractTarball_TotalSizeLimit(t *testing.T) {
	body := strings.Repeat("A", maxExtractSize)
	entries := make([]tarEntry, 60)
	for i := range entries {
		entries[i] = tarEntry{
			name: fmt.Sprintf("file%d.txt", i),
			body: body,
			mode: 0644,
			typeflag: tar.TypeReg,
		}
	}
	data := createTestTar(entries)
	dest := t.TempDir()
	files, err := ExtractTarball(bytes.NewReader(data), dest)
	if err != nil {
		t.Fatalf("ExtractTarball() error: %v", err)
	}
	if len(files) != 50 {
		t.Errorf("got %d files, want 50 (total size limit)", len(files))
	}
}

func TestProxyConcurrentRequests(t *testing.T) {
	safeTar := buildTestTarball(t, filepath.Join("..", "..", "testdata", "safe-pkg"))

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		w.Write(safeTar)
	}))
	defer upstream.Close()

	srv, err := New(Config{Upstream: upstream.URL, Workers: 2, CacheTTL: 0})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	proxyServer := httptest.NewServer(srv)
	defer proxyServer.Close()

	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func() {
			resp, err := http.Get(proxyServer.URL + "/safe/pkg/-/safe-1.0.0.tgz")
			if err != nil {
				errs <- err
				return
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				errs <- fmt.Errorf("status = %d", resp.StatusCode)
			} else {
				errs <- nil
			}
		}()
	}

	for i := 0; i < 10; i++ {
		if e := <-errs; e != nil {
			t.Error(e)
		}
	}
}

func BenchmarkExtractTarball(b *testing.B) {
	entries := make([]tarEntry, 100)
	for i := range entries {
		entries[i] = tarEntry{
			name: fmt.Sprintf("file%d.js", i),
			body: "console.log(1)",
			mode: 0644,
			typeflag: tar.TypeReg,
		}
	}
	data := createTestTar(entries)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dest, _ := os.MkdirTemp("", "bench-extract-")
		ExtractTarball(bytes.NewReader(data), dest)
		os.RemoveAll(dest)
	}
}
