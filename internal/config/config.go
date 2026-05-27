package config

import (
	"encoding/json"
	"os"
	"time"
)

type FileConfig struct {
	Threshold int            `json:"threshold"`
	Workers   int            `json:"workers"`
	Rules     map[string]int `json:"rules"`
	Proxy     struct {
		Port     int    `json:"port"`
		Upstream string `json:"upstream"`
	} `json:"proxy"`
	Cache struct {
		Enabled bool   `json:"enabled"`
		TTL     string `json:"ttl"`
	} `json:"cache"`
}

func Load(path string) (*FileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg FileConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *FileConfig) CacheTTL() time.Duration {
	if !c.Cache.Enabled {
		return 0
	}
	d, _ := time.ParseDuration(c.Cache.TTL)
	return d
}
