package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Config struct {
	Miniflux  MinifluxConfig
	Cache     CacheConfig
	Converter ConverterConfig
}

type MinifluxConfig struct {
	URL      string
	APIToken string
	Timeout  time.Duration
}

type CacheConfig struct {
	Mode    CacheMode
	TTL     time.Duration
	MaxSize int64  // Maximum cache size in bytes
	Path    string // Disk cache path
}

type CacheMode string

const (
	CacheMemory CacheMode = "memory"
	CacheDisk   CacheMode = "disk"
	CacheBoth   CacheMode = "both"
)

var validCacheModes = map[CacheMode]bool{
	CacheMemory: true,
	CacheDisk:   true,
	CacheBoth:   true,
}

type ConverterConfig struct {
	Timeout time.Duration
}

func LoadConfig() (*Config, error) {
	defaultCachePath := filepath.Join(os.TempDir(), "miniflux-mcp-cache")
	cfg := &Config{
		Miniflux: MinifluxConfig{
			URL:      getEnv("MINIFLUX_URL", "http://localhost"),
			APIToken: getEnv("MINIFLUX_API_TOKEN", ""),
			Timeout:  getDurationEnv("MINIFLUX_TIMEOUT", 30*time.Second),
		},
		Cache: CacheConfig{
			Mode:    CacheMode(getEnv("MCP_CACHE_MODE", "disk")),
			TTL:     getDurationEnv("MCP_CACHE_TTL", 24*time.Hour),
			MaxSize: getInt64Env("MCP_CACHE_MAX_SIZE", 1024*1024*1024), // 1GB default
			Path:    getEnv("MCP_CACHE_PATH", defaultCachePath),
		},
		Converter: ConverterConfig{
			Timeout: getDurationEnv("CONVERTER_TIMEOUT", 30*time.Second),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.Miniflux.URL == "" {
		return fmt.Errorf("MINIFLUX_URL is required")
	}
	if c.Miniflux.APIToken == "" {
		return fmt.Errorf("MINIFLUX_API_TOKEN is required")
	}
	if !validCacheModes[c.Cache.Mode] {
		return fmt.Errorf("invalid cache mode: %s", c.Cache.Mode)
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getInt64Env(key string, defaultValue int64) int64 {
	if value, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
