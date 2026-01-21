package services

import (
	"testing"
)

func TestExtractVideoID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		// Standard watch URLs
		{"standard watch URL", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"watch URL with extra params", "https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=120", "dQw4w9WgXcQ"},
		{"watch URL params before v", "https://www.youtube.com/watch?list=PLxyz&v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"http URL", "http://www.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"no www", "https://youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},

		// Short URLs
		{"short URL", "https://youtu.be/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"short URL with params", "https://youtu.be/dQw4w9WgXcQ?t=60", "dQw4w9WgXcQ"},

		// Embed URLs
		{"embed URL", "https://www.youtube.com/embed/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"embed URL with params", "https://www.youtube.com/embed/dQw4w9WgXcQ?autoplay=1", "dQw4w9WgXcQ"},

		// Shorts URLs
		{"shorts URL", "https://www.youtube.com/shorts/dQw4w9WgXcQ", "dQw4w9WgXcQ"},

		// v/ URLs
		{"v URL", "https://www.youtube.com/v/dQw4w9WgXcQ", "dQw4w9WgXcQ"},

		// Edge cases - these extract the valid ID portion even if there's extra text
		{"URL in text", "Check out this video: https://www.youtube.com/watch?v=dQw4w9WgXcQ it's great", "dQw4w9WgXcQ"},
		{"different video ID characters", "https://www.youtube.com/watch?v=a1B2c3D4e5F", "a1B2c3D4e5F"},
		{"underscore in ID", "https://www.youtube.com/watch?v=a_B-c3D4e5F", "a_B-c3D4e5F"},
		{"extra text after valid ID", "https://www.youtube.com/watch?v=dQw4w9WgXcQextra", "dQw4w9WgXcQ"},

		// Invalid inputs
		{"empty string", "", ""},
		{"no video ID", "https://www.youtube.com/", ""},
		{"invalid ID too short", "https://www.youtube.com/watch?v=abc", ""},
		{"not youtube", "https://vimeo.com/123456789", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractVideoID(tt.input)
			if got != tt.want {
				t.Errorf("ExtractVideoID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractVideoIDs(t *testing.T) {
	tests := []struct {
		name   string
		inputs []string
		want   []string
	}{
		{
			name: "multiple URLs",
			inputs: []string{
				"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
				"https://youtu.be/xvFZjo5PgG0",
				"https://www.youtube.com/embed/oHg5SJYRHA0",
			},
			want: []string{"dQw4w9WgXcQ", "xvFZjo5PgG0", "oHg5SJYRHA0"},
		},
		{
			name: "duplicates removed",
			inputs: []string{
				"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
				"https://youtu.be/dQw4w9WgXcQ",
				"https://www.youtube.com/embed/dQw4w9WgXcQ",
			},
			want: []string{"dQw4w9WgXcQ"},
		},
		{
			name: "mixed valid and invalid",
			inputs: []string{
				"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
				"https://example.com/not-youtube",
				"https://youtu.be/xvFZjo5PgG0",
			},
			want: []string{"dQw4w9WgXcQ", "xvFZjo5PgG0"},
		},
		{
			name: "multiple IDs in single string",
			inputs: []string{
				"Check out https://www.youtube.com/watch?v=dQw4w9WgXcQ and https://youtu.be/xvFZjo5PgG0",
			},
			want: []string{"dQw4w9WgXcQ", "xvFZjo5PgG0"},
		},
		{
			name:   "empty input",
			inputs: []string{},
			want:   nil,
		},
		{
			name:   "no valid URLs",
			inputs: []string{"not a url", "https://example.com"},
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractVideoIDs(tt.inputs)

			if len(got) != len(tt.want) {
				t.Errorf("ExtractVideoIDs() returned %d results, want %d", len(got), len(tt.want))
				return
			}

			for i, id := range got {
				if id != tt.want[i] {
					t.Errorf("ExtractVideoIDs()[%d] = %q, want %q", i, id, tt.want[i])
				}
			}
		})
	}
}

func TestIsValidVideoID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"dQw4w9WgXcQ", true},
		{"a1B2c3D4e5F", true},
		{"a_B-c3D4e5F", true},
		{"___________", true},
		{"-----------", true},

		// Invalid
		{"", false},
		{"dQw4w9WgXc", false},   // too short (10 chars)
		{"dQw4w9WgXcQQ", false}, // too long (12 chars)
		{"dQw4w9WgXc!", false},  // invalid character
		{"dQw4w9WgXc ", false},  // space
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := IsValidVideoID(tt.id)
			if got != tt.want {
				t.Errorf("IsValidVideoID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestWebSearchCache(t *testing.T) {
	// Create temp cache directory
	cache, err := NewWebSearchCache(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Test set and get
	results := []SearchResult{
		{VideoID: "abc123def45", URL: "https://youtube.com/watch?v=abc123def45"},
		{VideoID: "xyz789ghi01", URL: "https://youtube.com/watch?v=xyz789ghi01"},
	}

	key := "testkey123"
	if err := cache.Set(key, results); err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	// Retrieve
	got, found := cache.Get(key)
	if !found {
		t.Error("Expected to find cached results")
	}
	if len(got) != len(results) {
		t.Errorf("Got %d results, want %d", len(got), len(results))
	}
	for i, r := range got {
		if r.VideoID != results[i].VideoID {
			t.Errorf("Result[%d].VideoID = %q, want %q", i, r.VideoID, results[i].VideoID)
		}
	}

	// Test non-existent key
	_, found = cache.Get("nonexistent")
	if found {
		t.Error("Should not find non-existent key")
	}

	// Test clear
	if err := cache.Clear(); err != nil {
		t.Fatalf("Failed to clear cache: %v", err)
	}
	_, found = cache.Get(key)
	if found {
		t.Error("Should not find key after clear")
	}
}
