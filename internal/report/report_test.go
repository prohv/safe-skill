package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenID(t *testing.T) {
	id := genID()
	if len(id) != 36 {
		t.Errorf("genID() len = %d, want 36", len(id))
	}
	parts := strings.Split(id, "-")
	if len(parts) != 5 {
		t.Errorf("genID() = %q, want 5 dash-separated groups", id)
	}
	if parts[2][0] != '4' {
		t.Errorf("genID() version = %c, want '4'", parts[2][0])
	}
	v := parts[3][0]
	if v != '8' && v != '9' && v != 'a' && v != 'b' {
		t.Errorf("genID() variant = %c, want 8/9/a/b", v)
	}
}

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	r := New(nil, 0, "SAFE")

	if err := Save(dir, r); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	fpath := filepath.Join(dir, r.ID+".json")
	if _, err := os.Stat(fpath); err != nil {
		t.Fatalf("report file not found: %v", err)
	}

	loaded, err := Load(dir, r.ID)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.ID != r.ID || loaded.Risk != r.Risk || loaded.Status != r.Status {
		t.Errorf("round-trip mismatch: got %+v, want %+v", loaded, r)
	}
}

func TestLoadNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(dir, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent report")
	}
}
