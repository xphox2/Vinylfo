package duration

import (
	"context"
	"testing"
	"time"
)

func TestMusicBrainzClient_SearchTrack(t *testing.T) {
	client := NewMusicBrainzClient("test@example.com")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result, err := client.SearchTrack(ctx, "Bohemian Rhapsody", "Queen", "A Night at the Opera")

	if err == nil {
		t.Log("SearchTrack completed (may have hit real API or timeout)")
		if result != nil {
			t.Logf("Got result: %s by %s - %ds", result.Title, result.Artist, result.Duration)
		}
	} else {
		t.Logf("Expected error due to timeout/network: %v", err)
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
			expected: `recording:"Title" AND artist:"Artist"`,
		},
		{
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
	score := CalculateMatchScore("Song Title", "Machine Gun Kelly", "Song Title", "Machine Gun Kelly (2)")
	if score < 0.99 {
		t.Errorf("Expected score >= 0.99 for artist with (2) suffix, got %f", score)
	}

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
	score := CalculateMatchScore("99 Luftballons (Remastered)", "Nena", "99 Luftballons", "Nena")
	if score < 0.99 {
		t.Errorf("Expected score >= 0.99 for title with (Remastered) suffix, got %f", score)
	}

	score = CalculateMatchScore("Album", "Artist", "Album (Deluxe Edition)", "Artist")
	if score < 0.99 {
		t.Errorf("Expected score >= 0.99 for title with (Deluxe Edition) suffix, got %f", score)
	}
}
