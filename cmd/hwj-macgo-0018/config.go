package main

import (
	"os"
	"path/filepath"
)

// Config holds the runtime configuration for the service.
type Config struct {
	DataDir    string
	LogLevel   string
	MaxWorkers int
}

// LoadConfig builds a configuration from the given data directory.
func LoadConfig(dataDir string) *Config {
	cfg := &Config{
		DataDir:    dataDir,
		LogLevel:   "info",
		MaxWorkers: 4,
	}
	if v := os.Getenv("HWJ_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	return cfg
}

// EnsureDataDir creates the data directory and any parent directories if they
// do not already exist. It returns the resolved absolute path.
func EnsureDataDir(dataDir string) (string, error) {
	abs, err := filepath.Abs(dataDir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return "", err
	}
	return abs, nil
}
