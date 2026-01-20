package config

import (
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

func init() {
	_ = godotenv.Load()
}

type HTTPConfig struct {
	DefaultTimeout  time.Duration `env:"HTTP_DEFAULT_TIMEOUT" envDefault:"30s"`
	DiscogsTimeout  time.Duration `env:"HTTP_DISCOGS_TIMEOUT" envDefault:"60s"`
	DurationTimeout time.Duration `env:"HTTP_DURATION_TIMEOUT" envDefault:"30s"`
	ShutdownTimeout time.Duration `env:"HTTP_SHUTDOWN_TIMEOUT" envDefault:"10s"`
}

var HTTP = loadHTTPConfig()

func loadHTTPConfig() HTTPConfig {
	cfg := HTTPConfig{
		DefaultTimeout:  30 * time.Second,
		DiscogsTimeout:  60 * time.Second,
		DurationTimeout: 30 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	}

	if v := os.Getenv("HTTP_DEFAULT_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.DefaultTimeout = d
		}
	}

	if v := os.Getenv("HTTP_DISCOGS_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.DiscogsTimeout = d
		}
	}

	if v := os.Getenv("HTTP_DURATION_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.DurationTimeout = d
		}
	}

	if v := os.Getenv("HTTP_SHUTDOWN_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.ShutdownTimeout = d
		}
	}

	return cfg
}

func DefaultClient() *http.Client {
	return &http.Client{
		Timeout: HTTP.DefaultTimeout,
	}
}

func DiscogsClient() *http.Client {
	return &http.Client{
		Timeout: HTTP.DiscogsTimeout,
	}
}

func DurationClient() *http.Client {
	return &http.Client{
		Timeout: HTTP.DurationTimeout,
	}
}

func ShutdownClient() *http.Client {
	return &http.Client{
		Timeout: HTTP.ShutdownTimeout,
	}
}
