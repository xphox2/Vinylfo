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
			// Note: (Remix) is normalized out since it's an edition suffix
			title:    "Title (Remix)",
			artist:   "Artist",
			album:    "",
			expected: `recording:"Title" AND artist:"Artist"`,
		},
		{
			// (Part 1) is NOT an edition suffix, so it should be escaped
			title:    "Title (Part 1)",
			artist:   "Artist",
			album:    "",
			expected: `recording:"Title \(Part 1\)" AND artist:"Artist"`,
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
		// (Remastered) is now normalized, so this should score 1.0
		{"Bohemian Rhapsody", "Queen", "Bohemian Rhapsody (Remastered)", "Queen", 0.99, 1.0},
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

func TestNormalizeArtistName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"The Beatles", "The Beatles"},
		{"Machine Gun Kelly (2)", "Machine Gun Kelly"},
		{"Machine Gun Kelly (3)", "Machine Gun Kelly"},
		{"Artist (rapper)", "Artist"},
		{"Artist (singer)", "Artist"},
		{"Artist (band)", "Artist"},
		{"Artist (DJ)", "Artist"},
		{"Artist (dj)", "Artist"},
		{"Artist (musician)", "Artist"},
		{"Artist (producer)", "Artist"},
		{"Artist (artist)", "Artist"},
		{"Queen", "Queen"},
		{"The Beatles (2)", "The Beatles"},
		// Ensure we don't strip legitimate parenthetical content
		{"!!! (band)", "!!!"},
		{"Pink Floyd", "Pink Floyd"},
	}

	for _, tc := range tests {
		result := NormalizeArtistName(tc.input)
		if result != tc.expected {
			t.Errorf("NormalizeArtistName(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestCalculateMatchScore_WithDisambiguation(t *testing.T) {
	// Test that "(2)" suffix doesn't affect match score
	score := CalculateMatchScore("Song Title", "Machine Gun Kelly", "Song Title", "Machine Gun Kelly (2)")
	if score < 0.99 {
		t.Errorf("Expected score >= 0.99 for artist with (2) suffix, got %f", score)
	}

	// Test that "(rapper)" suffix doesn't affect match score
	score = CalculateMatchScore("Song Title", "Artist", "Song Title", "Artist (rapper)")
	if score < 0.99 {
		t.Errorf("Expected score >= 0.99 for artist with (rapper) suffix, got %f", score)
	}
}

func TestNormalizeTitle(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"99 Luftballons", "99 Luftballons"},
		{"99 Luftballons (Remastered)", "99 Luftballons"},
		{"99 Luftballons (Digital Remaster)", "99 Luftballons"},
		{"Nena (Remastered & Selected Works)", "Nena"},
		{"Album (Deluxe Edition)", "Album"},
		{"Album (Deluxe)", "Album"},
		{"Album (Special Edition)", "Album"},
		{"Album (Bonus Track Version)", "Album"},
		{"Album (Anniversary Edition)", "Album"},
		{"Album (Expanded Edition)", "Album"},
		{"The Great Hits (Remastered)", "The Great Hits"},
		{"Album (2020 Remaster)", "Album"},
		{"Album (Mono Version)", "Album"},
		{"Album (Stereo Mix)", "Album"},
		{"Album (Original Mix)", "Album"},
		{"Album (Enhanced)", "Album"},
		// Should NOT strip - not an edition suffix
		{"Album (Part 1)", "Album (Part 1)"},
		{"The Wall", "The Wall"},
	}

	for _, tc := range tests {
		result := NormalizeTitle(tc.input)
		if result != tc.expected {
			t.Errorf("NormalizeTitle(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestCalculateMatchScore_WithEditionSuffix(t *testing.T) {
	// Test that "(Remastered)" suffix doesn't affect match score
	score := CalculateMatchScore("99 Luftballons (Remastered)", "Nena", "99 Luftballons", "Nena")
	if score < 0.99 {
		t.Errorf("Expected score >= 0.99 for title with (Remastered) suffix, got %f", score)
	}

	// Test that "(Deluxe Edition)" suffix doesn't affect match score
	score = CalculateMatchScore("Album", "Artist", "Album (Deluxe Edition)", "Artist")
	if score < 0.99 {
		t.Errorf("Expected score >= 0.99 for title with (Deluxe Edition) suffix, got %f", score)
	}
}
