package duration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMusicBrainzClient_SearchTrack(t *testing.T) {
	mockResponse := `{
		"created": "2024-01-15T10:30:00.000Z",
		"count": 1,
		"offset": 0,
		"recordings": [
			{
				"id": "test-recording-id",
				"score": 95,
				"title": "Bohemian Rhapsody",
				"length": 354000,
				"artist-credit": [
					{
						"name": "Queen",
						"artist": {
							"id": "artist-id",
							"name": "Queen"
						}
					}
				],
				"releases": [
					{
						"id": "release-id",
						"title": "A Night at the Opera",
						"date": "1975-11-21"
					}
				]
			}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("User-Agent header is required")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	client := NewMusicBrainzClient("test@example.com")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = ctx
	_ = client

	query := client.buildQuery("Bohemian Rhapsody", "Queen", "A Night at the Opera")
	expected := `recording:"Bohemian Rhapsody" AND artist:"Queen" AND release:"A Night at the Opera"`
	if query != expected {
		t.Errorf("Query mismatch:\ngot:  %s\nwant: %s", query, expected)
	}
}

func TestMusicBrainzClient_buildQuery_EscapesSpecialChars(t *testing.T) {
	client := NewMusicBrainzClient("test@example.com")

	tests := []struct {
		title    string
		artist   string
		album    string
		expected string
	}{
		{
			title:    "What's Going On",
			artist:   "Marvin Gaye",
			album:    "",
			expected: `recording:"What's Going On" AND artist:"Marvin Gaye"`,
		},
		{
			title:    "Rock & Roll",
			artist:   "Led Zeppelin",
			album:    "Led Zeppelin IV",
			expected: `recording:"Rock & Roll" AND artist:"Led Zeppelin" AND release:"Led Zeppelin IV"`,
		},
		{
			title:    "Title (Remix)",
			artist:   "Artist",
			album:    "",
			expected: `recording:"Title \(Remix\)" AND artist:"Artist"`,
		},
	}

	for _, tc := range tests {
		result := client.buildQuery(tc.title, tc.artist, tc.album)
		if result != tc.expected {
			t.Errorf("buildQuery(%q, %q, %q):\ngot:  %s\nwant: %s",
				tc.title, tc.artist, tc.album, result, tc.expected)
		}
	}
}

func TestCalculateMatchScore(t *testing.T) {
	tests := []struct {
		searchTitle  string
		searchArtist string
		resultTitle  string
		resultArtist string
		minScore     float64
		maxScore     float64
	}{
		{"Bohemian Rhapsody", "Queen", "Bohemian Rhapsody", "Queen", 0.99, 1.0},
		{"bohemian rhapsody", "queen", "Bohemian Rhapsody", "Queen", 0.99, 1.0},
		{"Bohemian Rhapsody", "Queen", "Bohemian Rhapsody (Remastered)", "Queen", 0.7, 0.95},
		{"Bohemian Rhapsody", "Queen", "We Will Rock You", "Queen", 0.0, 0.6},
		{"Song", "Artist A", "Song", "Artist B", 0.7, 1.0},
	}

	for _, tc := range tests {
		score := CalculateMatchScore(tc.searchTitle, tc.searchArtist, tc.resultTitle, tc.resultArtist)
		if score < tc.minScore || score > tc.maxScore {
			t.Errorf("CalculateMatchScore(%q, %q, %q, %q) = %f, want between %f and %f",
				tc.searchTitle, tc.searchArtist, tc.resultTitle, tc.resultArtist,
				score, tc.minScore, tc.maxScore)
		}
	}
}

func TestMusicBrainzClient_Name(t *testing.T) {
	client := NewMusicBrainzClient("test@example.com")
	if client.Name() != "musicbrainz" {
		t.Errorf("Expected name 'musicbrainz', got '%s'", client.Name())
	}
}

func TestMusicBrainzClient_IsConfigured(t *testing.T) {
	client := NewMusicBrainzClient("test@example.com")
	if !client.IsConfigured() {
		t.Error("Expected client to be configured with valid email")
	}

	emptyClient := NewMusicBrainzClient("")
	if emptyClient.IsConfigured() {
		t.Error("Expected client to not be configured with empty email")
	}
}
