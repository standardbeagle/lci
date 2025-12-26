package config

import (
	"os"
	"strconv"
)

// Config holds application configuration
type Config struct {
	Port        int    `json:"port"`
	DatabaseURL string `json:"database_url"`
	JWTSecret   string `json:"jwt_secret"`
	Debug       bool   `json:"debug"`
	CORS        CORS   `json:"cors"`
}

// CORS holds CORS configuration
type CORS struct {
	AllowedOrigins []string `json:"allowed_origins"`
	AllowedMethods []string `json:"allowed_methods"`
	AllowedHeaders []string `json:"allowed_headers"`
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	config := &Config{
		Port:        getEnvInt("PORT", 8080),
		DatabaseURL: getEnvString("DATABASE_URL", "postgres://localhost/app_dev"),
		JWTSecret:   getEnvString("JWT_SECRET", "default-secret"),
		Debug:       getEnvBool("DEBUG", false),
		CORS: CORS{
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"Content-Type", "Authorization"},
		},
	}

	return config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Port)
	}

	if c.DatabaseURL == "" {
		return fmt.Errorf("database URL is required")
	}

	if c.JWTSecret == "" {
		return fmt.Errorf("JWT secret is required")
	}

	return nil
}

// Helper functions
func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
