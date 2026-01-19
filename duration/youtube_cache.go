package duration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type YouTubeCacheEntry struct {
	Query      string    `json:"query"`
	Duration   int       `json:"duration"`
	VideoID    string    `json:"video_id"`
	VideoTitle string    `json:"video_title"`
	MatchScore float64   `json:"match_score"`
	CachedAt   time.Time `json:"cached_at"`
}

type YouTubeCache struct {
	cacheDir string
	mu       sync.RWMutex
	ttl      time.Duration
}

func NewYouTubeCache(cacheDir string) (*YouTubeCache, error) {
	if cacheDir == "" {
		cacheDir = filepath.Join(".", ".youtube_cache")
	}

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &YouTubeCache{
		cacheDir: cacheDir,
		ttl:      30 * 24 * time.Hour, // 30 days default TTL
	}, nil
}

func (c *YouTubeCache) generateKey(title, artist, album string) string {
	normalized := strings.ToLower(strings.TrimSpace(title + "|" + artist + "|" + album))
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:16]) // Use first 16 bytes for shorter filenames
}

func (c *YouTubeCache) getCachePath(key string) string {
	return filepath.Join(c.cacheDir, key+".json")
}

func (c *YouTubeCache) Get(title, artist, album string) (*YouTubeCacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := c.generateKey(title, artist, album)
	path := c.getCachePath(key)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}

	var entry YouTubeCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false
	}

	// Check TTL
	if time.Since(entry.CachedAt) > c.ttl {
		os.Remove(path) // Clean up expired entry
		return nil, false
	}

	return &entry, true
}

func (c *YouTubeCache) Set(title, artist, album string, result *TrackSearchResult) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if result == nil {
		return nil
	}

	key := c.generateKey(title, artist, album)
	entry := YouTubeCacheEntry{
		Query:      title + " - " + artist,
		Duration:   result.Duration,
		VideoID:    result.ExternalID,
		VideoTitle: result.Title,
		MatchScore: result.MatchScore,
		CachedAt:   time.Now(),
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	path := c.getCachePath(key)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

func (c *YouTubeCache) SetNotFound(title, artist, album string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := c.generateKey(title, artist, album)
	entry := YouTubeCacheEntry{
		Query:      title + " - " + artist,
		Duration:   -1, // Sentinel value for "not found"
		CachedAt:   time.Now(),
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	path := c.getCachePath(key)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

func (c *YouTubeCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			os.Remove(filepath.Join(c.cacheDir, entry.Name()))
		}
	}

	return nil
}

func (c *YouTubeCache) Stats() (total int, expired int, err error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		return 0, 0, err
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		total++

		path := filepath.Join(c.cacheDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var cacheEntry YouTubeCacheEntry
		if err := json.Unmarshal(data, &cacheEntry); err != nil {
			continue
		}

		if time.Since(cacheEntry.CachedAt) > c.ttl {
			expired++
		}
	}

	return total, expired, nil
}
