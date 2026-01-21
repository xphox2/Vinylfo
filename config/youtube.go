package config

import (
	"os"
	"strconv"
)

type YouTubeMatchConfig struct {
	MatchScoreThreshold float64 `env:"YOUTUBE_MATCH_SCORE_THRESHOLD" envDefault:"0.6"`
	AutoMatchThreshold  float64 `env:"YOUTUBE_AUTO_MATCH_THRESHOLD" envDefault:"0.85"`
	MaxCandidates       int     `env:"YOUTUBE_MAX_CANDIDATES" envDefault:"5"`
	WebSearchEnabled    bool    `env:"YOUTUBE_WEB_SEARCH_ENABLED" envDefault:"true"`
	APIFallbackEnabled  bool    `env:"YOUTUBE_API_FALLBACK_ENABLED" envDefault:"true"`
}

var YouTubeMatch = loadYouTubeMatchConfig()

func loadYouTubeMatchConfig() YouTubeMatchConfig {
	cfg := YouTubeMatchConfig{
		MatchScoreThreshold: 0.6,
		AutoMatchThreshold:  0.85,
		MaxCandidates:       5,
		WebSearchEnabled:    true,
		APIFallbackEnabled:  true,
	}

	if v := os.Getenv("YOUTUBE_MATCH_SCORE_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.MatchScoreThreshold = f
		}
	}

	if v := os.Getenv("YOUTUBE_AUTO_MATCH_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.AutoMatchThreshold = f
		}
	}

	if v := os.Getenv("YOUTUBE_MAX_CANDIDATES"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.MaxCandidates = i
		}
	}

	if v := os.Getenv("YOUTUBE_WEB_SEARCH_ENABLED"); v != "" {
		cfg.WebSearchEnabled = v == "true" || v == "1"
	}

	if v := os.Getenv("YOUTUBE_API_FALLBACK_ENABLED"); v != "" {
		cfg.APIFallbackEnabled = v == "true" || v == "1"
	}

	return cfg
}
