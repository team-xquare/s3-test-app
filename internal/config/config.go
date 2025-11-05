package config

import (
	"fmt"
	"os"
)

// Config holds the application configuration
type Config struct {
	Server   ServerConfig
	S3       S3Config
	Log      LogConfig
	Database DatabaseConfig
	Auth     AuthConfig
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port string
	Host string
}

// S3Config holds S3/MinIO configuration
type S3Config struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
}

// LogConfig holds logging configuration
type LogConfig struct {
	Level string
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Path string
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Secret string
}

// NewConfig creates a new configuration from environment variables
func NewConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("PORT", "8080"),
			Host: getEnv("HOST", "0.0.0.0"),
		},
		S3: S3Config{
			Endpoint:  getEnv("S3_ENDPOINT", "http://xquare-test:8333"),
			Region:    getEnv("S3_REGION", "us-east-1"),
			Bucket:    getEnv("S3_BUCKET", "test-buckets"),
			AccessKey: getEnv("S3_ACCESS_KEY", ""),
			SecretKey: getEnv("S3_SECRET_KEY", ""),
		},
		Log: LogConfig{
			Level: getEnv("LOG_LEVEL", "info"),
		},
		Database: DatabaseConfig{
			Path: getEnv("DB_PATH", "./data/app.db"),
		},
		Auth: AuthConfig{
			Secret: getEnv("AUTH_SECRET", "change-this-secret-in-production"),
		},
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.S3.AccessKey == "" {
		return fmt.Errorf("S3_ACCESS_KEY is required")
	}
	if c.S3.SecretKey == "" {
		return fmt.Errorf("S3_SECRET_KEY is required")
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}