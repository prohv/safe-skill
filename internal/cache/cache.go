package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"safeskill/internal/report"
)

type Config struct {
	TTL time.Duration
}

type Cache struct {
	dir string
	ttl time.Duration
}

type entry struct {
	Hash      string         `json:"hash"`
	Report    *report.Report `json:"report"`
	Timestamp int64          `json:"timestamp"`
}

func New(dir string, cfg Config) *Cache {
	c := &Cache{dir: dir, ttl: cfg.TTL}
	os.MkdirAll(dir, 0755)
	c.Prune()
	return c
}

func (c *Cache) Check(hash string) (*report.Report, bool) {
	if c.ttl <= 0 {
		return nil, false
	}
	data, err := os.ReadFile(filepath.Join(c.dir, hash+".json"))
	if err != nil {
		return nil, false
	}
	var e entry
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, false
	}
	if time.Now().Unix()-e.Timestamp > int64(c.ttl.Seconds()) {
		os.Remove(filepath.Join(c.dir, hash+".json"))
		return nil, false
	}
	return e.Report, true
}

func (c *Cache) Store(hash string, r *report.Report) error {
	if c.ttl <= 0 {
		return nil
	}
	if err := os.MkdirAll(c.dir, 0755); err != nil {
		return fmt.Errorf("cache: %w", err)
	}
	e := entry{Hash: hash, Report: r, Timestamp: time.Now().Unix()}
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return fmt.Errorf("cache: %w", err)
	}
	return os.WriteFile(filepath.Join(c.dir, hash+".json"), data, 0644)
}

func (c *Cache) Prune() {
	if c.ttl <= 0 {
		return
	}
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(c.dir, e.Name()))
		if err != nil {
			continue
		}
		var entry entry
		if json.Unmarshal(data, &entry) != nil {
			continue
		}
		if time.Now().Unix()-entry.Timestamp > int64(c.ttl.Seconds()) {
			os.Remove(filepath.Join(c.dir, e.Name()))
		}
	}
}

func Hash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
