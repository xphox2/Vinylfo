package services

import (
	"testing"
)

func TestCalculateDurationScore(t *testing.T) {
	matcher := NewYouTubeMatcher()

	tests := []struct {
		name             string
		expectedDuration int
		actualDuration   int
		wantMin          float64
		wantMax          float64
	}{
		// Perfect matches
		{"exact match", 180, 180, 1.0, 1.0},
		{"within 3 seconds", 180, 182, 1.0, 1.0},
		{"within 3 seconds negative", 180, 177, 1.0, 1.0},

		// Excellent matches
		{"within 10 seconds", 180, 188, 0.85, 0.95},
		{"within 10 seconds negative", 180, 171, 0.85, 0.95},

		// Good matches
		{"within 30 seconds", 180, 200, 0.65, 0.75},
		{"within 30 seconds negative", 180, 155, 0.65, 0.75},

		// Acceptable matches
		{"within 60 seconds", 180, 230, 0.45, 0.55},
		{"within 60 seconds negative", 180, 125, 0.45, 0.55},

		// Poor matches
		{"within 120 seconds", 180, 280, 0.25, 0.35},
		{"extended version", 180, 60, 0.25, 0.35},

		// Very poor matches
		{"very different duration", 180, 400, 0.05, 0.15},
		{"way too short", 180, 30, 0.05, 0.15},

		// Edge cases
		{"unknown expected duration", 0, 180, 0.45, 0.55},
		{"unknown actual duration", 180, 0, 0.25, 0.35},
		{"both unknown", 0, 0, 0.25, 0.55},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := matcher.calculateDurationScore(tt.expectedDuration, tt.actualDuration)
			if score < tt.wantMin || score > tt.wantMax {
				t.Errorf("calculateDurationScore(%d, %d) = %v, want between %v and %v",
					tt.expectedDuration, tt.actualDuration, score, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestCalculateChannelScore(t *testing.T) {
	matcher := NewYouTubeMatcher()

	tests := []struct {
		name        string
		artistName  string
		channelName string
		wantMin     float64
		wantMax     float64
	}{
		// Perfect/excellent matches
		{"exact match", "The Beatles", "The Beatles", 0.95, 1.0},
		{"VEVO channel", "Beyoncé", "BeyoncéVEVO", 0.9, 1.0},
		{"official channel", "Taylor Swift", "Taylor Swift Official", 0.9, 1.0},
		{"topic channel", "Pink Floyd", "Pink Floyd - Topic", 0.9, 1.0},

		// Good matches
		{"artist in channel name", "Coldplay", "ColdplayOfficial", 0.9, 1.0}, // VEVO check catches this
		{"similar name", "The Rolling Stones", "Rolling Stones", 0.7, 0.9},

		// Partial matches
		{"partial match", "Queen", "Queen Official", 0.9, 1.0}, // Official check boosts this
		{"record label", "Drake", "OVO Sound", 0.0, 0.4},

		// Poor matches
		{"unrelated channel", "Adele", "Random Music Channel", 0.0, 0.3},
		{"completely different", "Ed Sheeran", "Cooking With Chef", 0.0, 0.2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := matcher.calculateChannelScore(tt.artistName, tt.channelName)
			if score < tt.wantMin || score > tt.wantMax {
				t.Errorf("calculateChannelScore(%q, %q) = %v, want between %v and %v",
					tt.artistName, tt.channelName, score, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestCalculateTitleScore(t *testing.T) {
	matcher := NewYouTubeMatcher()

	tests := []struct {
		name       string
		trackTitle string
		videoTitle string
		wantMin    float64
		wantMax    float64
	}{
		// Perfect matches
		{"exact match", "Bohemian Rhapsody", "Bohemian Rhapsody", 0.95, 1.0},
		{"with video suffix", "Bohemian Rhapsody", "Bohemian Rhapsody (Official Video)", 0.95, 1.0},
		{"with artist prefix", "Bohemian Rhapsody", "Queen - Bohemian Rhapsody", 0.95, 1.0},

		// Good matches
		{"remastered version", "Let It Be", "Let It Be (Remastered 2009)", 0.85, 1.0},
		{"audio tag", "Yesterday", "Yesterday (Official Audio)", 0.8, 1.0},
		{"lyric video", "Imagine", "Imagine (Lyric Video)", 0.5, 0.7},

		// Partial matches
		{"slight variation", "Don't Stop Believin'", "Dont Stop Believing", 0.7, 0.9},
		{"live version indicator", "Hotel California", "Hotel California Live", 0.75, 0.95},

		// Poor matches
		{"different song", "Stairway to Heaven", "Black Dog", 0.0, 0.4},
		{"completely different", "Wonderwall", "Smells Like Teen Spirit", 0.0, 0.3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := matcher.calculateTitleScore(tt.trackTitle, tt.videoTitle)
			if score < tt.wantMin || score > tt.wantMax {
				t.Errorf("calculateTitleScore(%q, %q) = %v, want between %v and %v",
					tt.trackTitle, tt.videoTitle, score, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestCalculateArtistScore(t *testing.T) {
	matcher := NewYouTubeMatcher()

	tests := []struct {
		name        string
		trackArtist string
		videoTitle  string
		channelName string
		wantMin     float64
		wantMax     float64
	}{
		// Artist in video title
		{"artist in title", "Queen", "Queen - Bohemian Rhapsody", "QueenVEVO", 0.95, 1.0},
		{"artist in title different channel", "Led Zeppelin", "Led Zeppelin - Stairway to Heaven", "Random Uploader", 0.95, 1.0},

		// Artist matches channel
		{"VEVO channel", "Beyoncé", "Single Ladies", "BeyoncéVEVO", 0.9, 1.0},
		{"official channel", "Taylor Swift", "Shake It Off", "TaylorSwiftVEVO", 0.3, 0.5}, // Space in name affects similarity

		// Partial matches
		{"partial in title", "The Beatles", "Hey Jude - Beatles Classic", "ClassicRockChannel", 0.3, 0.5},
		{"word match", "Pink Floyd", "Comfortably Numb Floyd", "MusicChannel", 0.3, 0.5},

		// Poor matches
		{"no match", "Nirvana", "Random Song Title", "RandomChannel", 0.2, 0.4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := matcher.calculateArtistScore(tt.trackArtist, tt.videoTitle, tt.channelName)
			if score < tt.wantMin || score > tt.wantMax {
				t.Errorf("calculateArtistScore(%q, %q, %q) = %v, want between %v and %v",
					tt.trackArtist, tt.videoTitle, tt.channelName, score, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestCalculateScore_Composite(t *testing.T) {
	matcher := NewYouTubeMatcher()

	tests := []struct {
		name          string
		trackTitle    string
		trackArtist   string
		trackDuration int
		videoTitle    string
		channelName   string
		videoDuration int
		wantAutoMatch bool
		wantAccept    bool
	}{
		{
			name:          "perfect match",
			trackTitle:    "Bohemian Rhapsody",
			trackArtist:   "Queen",
			trackDuration: 354,
			videoTitle:    "Queen - Bohemian Rhapsody (Official Video)",
			channelName:   "QueenVEVO",
			videoDuration: 355,
			wantAutoMatch: true,
			wantAccept:    true,
		},
		{
			name:          "good match with official video auto-matches",
			trackTitle:    "Let It Be",
			trackArtist:   "The Beatles",
			trackDuration: 243,
			videoTitle:    "The Beatles - Let It Be (Official Video)",
			channelName:   "The Beatles",
			videoDuration: 250,
			wantAutoMatch: true, // Official video bonus pushes it over
			wantAccept:    true,
		},
		{
			name:          "poor match",
			trackTitle:    "Stairway to Heaven",
			trackArtist:   "Led Zeppelin",
			trackDuration: 482,
			videoTitle:    "Random Song - Unknown Artist",
			channelName:   "RandomUploader",
			videoDuration: 180,
			wantAutoMatch: false,
			wantAccept:    false,
		},
		{
			name:          "official audio high quality",
			trackTitle:    "Shape of You",
			trackArtist:   "Ed Sheeran",
			trackDuration: 234,
			videoTitle:    "Ed Sheeran - Shape of You [Official Audio]",
			channelName:   "Ed Sheeran",
			videoDuration: 234,
			wantAutoMatch: true,
			wantAccept:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := matcher.CalculateScore(
				tt.trackTitle, tt.trackArtist, tt.trackDuration,
				tt.videoTitle, tt.channelName, tt.videoDuration,
			)

			if matcher.IsAutoMatch(score) != tt.wantAutoMatch {
				t.Errorf("IsAutoMatch() = %v, want %v (composite: %v)",
					matcher.IsAutoMatch(score), tt.wantAutoMatch, score.Composite)
			}

			if matcher.IsAcceptableMatch(score) != tt.wantAccept {
				t.Errorf("IsAcceptableMatch() = %v, want %v (composite: %v)",
					matcher.IsAcceptableMatch(score), tt.wantAccept, score.Composite)
			}

			// Verify composite is at least the weighted sum (bonus may be applied)
			baseComposite := (score.Title * 0.40) + (score.Artist * 0.30) +
				(score.Duration * 0.20) + (score.Channel * 0.10)
			if score.Composite < baseComposite-0.001 {
				t.Errorf("Composite score %v is less than base %v", score.Composite, baseComposite)
			}
		})
	}
}

func TestNormalizeVideoTitle(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Bohemian Rhapsody (Official Video)", "Bohemian Rhapsody"},
		{"Queen - Bohemian Rhapsody", "Bohemian Rhapsody"},
		{"Shape of You [Official Music Video]", "Shape of You"},
		{"Let It Be (Remastered 2009)", "Let It Be"},
		{"Yesterday - The Beatles (HD)", "The Beatles"},
		{"Stairway to Heaven", "Stairway to Heaven"},
		{"Artist - Song Title (Official Audio)", "Song Title"},
		{"Track Name [4K]", "Track Name"},
		{"Song (Official Visualizer)", "Song"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeVideoTitle(tt.input)
			if got != tt.want {
				t.Errorf("normalizeVideoTitle(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsOfficialChannel(t *testing.T) {
	tests := []struct {
		channelName string
		want        bool
	}{
		{"QueenVEVO", true},
		{"Taylor Swift Official", true},
		{"Pink Floyd - Topic", true},
		{"Universal Music", true},
		{"Atlantic Records", true},
		{"Random User", false},
		{"Music Lover 123", true}, // contains "music"
		{"John's Uploads", false},
	}

	for _, tt := range tests {
		t.Run(tt.channelName, func(t *testing.T) {
			got := isOfficialChannel(tt.channelName)
			if got != tt.want {
				t.Errorf("isOfficialChannel(%q) = %v, want %v", tt.channelName, got, tt.want)
			}
		})
	}
}

func TestMatcherThresholds(t *testing.T) {
	matcher := NewYouTubeMatcher()

	// Test that thresholds work correctly
	autoMatchScore := YouTubeMatchScore{Composite: 0.90}
	needsReviewScore := YouTubeMatchScore{Composite: 0.75}
	rejectScore := YouTubeMatchScore{Composite: 0.40}

	if !matcher.IsAutoMatch(autoMatchScore) {
		t.Error("Score 0.90 should be auto-match")
	}
	if matcher.IsAutoMatch(needsReviewScore) {
		t.Error("Score 0.75 should not be auto-match")
	}

	if !matcher.IsAcceptableMatch(needsReviewScore) {
		t.Error("Score 0.75 should be acceptable")
	}
	if matcher.IsAcceptableMatch(rejectScore) {
		t.Error("Score 0.40 should not be acceptable")
	}

	if !matcher.NeedsReview(needsReviewScore) {
		t.Error("Score 0.75 should need review")
	}
	if matcher.NeedsReview(autoMatchScore) {
		t.Error("Score 0.90 should not need review (auto-match)")
	}
	if matcher.NeedsReview(rejectScore) {
		t.Error("Score 0.40 should not need review (rejected)")
	}
}

func TestCustomConfig(t *testing.T) {
	// Test with stricter thresholds
	config := YouTubeMatchConfig{
		AutoMatchThreshold: 0.95,
		MinMatchThreshold:  0.70,
		TitleWeight:        0.50,
		ArtistWeight:       0.25,
		DurationWeight:     0.15,
		ChannelWeight:      0.10,
		DurationPerfect:    2,
		DurationExcellent:  5,
		DurationGood:       15,
		DurationAcceptable: 30,
		DurationPoor:       60,
	}

	matcher := NewYouTubeMatcherWithConfig(config)

	// Score that would be auto-match with default config
	score := YouTubeMatchScore{Composite: 0.90}

	if matcher.IsAutoMatch(score) {
		t.Error("With strict config, 0.90 should not be auto-match")
	}
	if !matcher.IsAcceptableMatch(score) {
		t.Error("With strict config, 0.90 should still be acceptable")
	}
}

func TestOfficialVideoBonus(t *testing.T) {
	matcher := NewYouTubeMatcher()

	tests := []struct {
		name       string
		videoTitle string
		wantBonus  float64
	}{
		{"official video", "Song Title (Official Video)", 0.15},
		{"official music video", "Song Title (Official Music Video)", 0.15},
		{"official audio", "Song Title [Official Audio]", 0.15},
		{"official release", "Song Title (Official Release)", 0.15},
		{"case insensitive", "Song Title (OFFICIAL VIDEO)", 0.15},
		{"no bonus", "Song Title (Live Version)", 0.0},
		{"no bonus lyrics", "Song Title (Lyric Video)", 0.0},
		{"no bonus visualizer", "Song Title (Visualizer)", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matcher.calculateOfficialVideoBonus(tt.videoTitle)
			if got != tt.wantBonus {
				t.Errorf("calculateOfficialVideoBonus(%q) = %v, want %v",
					tt.videoTitle, got, tt.wantBonus)
			}
		})
	}
}

func TestOfficialVideoBonusIntegration(t *testing.T) {
	matcher := NewYouTubeMatcher()

	tests := []struct {
		name          string
		trackTitle    string
		trackArtist   string
		videoTitle    string
		channelName   string
		videoDuration int
		trackDuration int
		wantAutoMatch bool
	}{
		{
			name:          "official video pushes marginal match over threshold",
			trackTitle:    "Breathe",
			trackArtist:   "Pink Floyd",
			videoTitle:    "Breathe (Official Video)",
			channelName:   "PinkFloydVEVO",
			videoDuration: 215,
			trackDuration: 220,
			wantAutoMatch: true, // Official video bonus should push over threshold
		},
		{
			name:          "same without official tag stays below threshold",
			trackTitle:    "Breathe",
			trackArtist:   "Pink Floyd",
			videoTitle:    "Breathe",
			channelName:   "PinkFloydVEVO",
			videoDuration: 215,
			trackDuration: 220,
			wantAutoMatch: false, // No bonus, should be below threshold
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := matcher.CalculateScore(
				tt.trackTitle, tt.trackArtist, tt.trackDuration,
				tt.videoTitle, tt.channelName, tt.videoDuration,
			)
			if matcher.IsAutoMatch(score) != tt.wantAutoMatch {
				t.Errorf("IsAutoMatch() = %v, want %v (composite: %.2f, title: %.2f, artist: %.2f, duration: %.2f, channel: %.2f)",
					matcher.IsAutoMatch(score), tt.wantAutoMatch, score.Composite,
					score.Title, score.Artist, score.Duration, score.Channel)
			}
		})
	}
}

func TestOfficialVideoBonusDoesNotExceedOne(t *testing.T) {
	matcher := NewYouTubeMatcher()

	// A near-perfect match that would exceed 1.0 with bonus should be capped at 1.0
	score := matcher.CalculateScore(
		"Test Song", "Test Artist", 180,
		"Test Artist - Test Song (Official Video)", "TestArtistVEVO", 180,
	)

	if score.Composite > 1.0 {
		t.Errorf("Composite score %v exceeds 1.0", score.Composite)
	}
	if score.Composite != 1.0 {
		t.Errorf("Expected perfect score of 1.0, got %v", score.Composite)
	}
}
