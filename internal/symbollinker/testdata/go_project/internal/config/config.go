package config

import (
	"os"
	"path/filepath"
)

// Config holds application configuration
type Config struct {
	AppName string
	Version string
}

// Load loads the configuration
func Load() *Config {
	return &Config{
		AppName: "TestApp",
		Version: "1.0.0",
	}
}

// GetConfigPath returns the config file path
func GetConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "app.conf")
}
