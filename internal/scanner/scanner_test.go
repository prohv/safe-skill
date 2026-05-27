package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"safeskill/internal/rules"
)

func TestIntegration(t *testing.T) {
	t.Run("safe package", func(t *testing.T) {
		root := filepath.Join("..", "..", "testdata", "safe-pkg")
		files, err := Walk(root)
		if err != nil {
			t.Fatalf("Walk() error: %v", err)
		}
		if len(files) == 0 {
			t.Fatal("Walk() returned no files for safe-pkg")
		}

		pool := NewPool(2, rules.BuiltinRules())
		results := pool.Run(files)
		signals, score := Aggregate(results)

		if score >= 10 {
			t.Errorf("safe package score = %d, want < 10, signals: %+v", score, signals)
		}
	})

	t.Run("suspicious package", func(t *testing.T) {
		root := filepath.Join("..", "..", "testdata", "suspicious-pkg")
		files, err := Walk(root)
		if err != nil {
			t.Fatalf("Walk() error: %v", err)
		}
		if len(files) == 0 {
			t.Fatal("Walk() returned no files for suspicious-pkg")
		}

		pool := NewPool(2, rules.BuiltinRules())
		results := pool.Run(files)
		signals, score := Aggregate(results)

		if score < 30 {
			t.Errorf("suspicious package score = %d, want >= 30, signals: %+v", score, signals)
		}
		if len(signals) == 0 {
			t.Error("suspicious package had no signals, expected some")
		}
	})
}

func TestWalk_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	files, err := Walk(dir)
	if err != nil {
		t.Fatalf("Walk() error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("Walk() got %d files in empty dir, want 0", len(files))
	}
}

func TestWalk_ManyFiles(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 500; i++ {
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%d.js", i)), []byte("x"), 0644)
	}
	files, err := Walk(dir)
	if err != nil {
		t.Fatalf("Walk() error: %v", err)
	}
	if len(files) != 500 {
		t.Errorf("Walk() got %d files, want 500", len(files))
	}
}

func BenchmarkWalk(b *testing.B) {
	root := filepath.Join("..", "..", "testdata", "suspicious-pkg")
	for i := 0; i < b.N; i++ {
		Walk(root)
	}
}

func BenchmarkPool(b *testing.B) {
	root := filepath.Join("..", "..", "testdata", "suspicious-pkg")
	files, _ := Walk(root)
	pool := NewPool(2, rules.BuiltinRules())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.Run(files)
	}
}
