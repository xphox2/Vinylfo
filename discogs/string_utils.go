package discogs

import (
	"net/url"
	"strings"
)

func normalizeStringForCompare(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "the ", "")
	s = strings.ReplaceAll(s, "&", "and")
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.ReplaceAll(s, "(", "")
	s = strings.ReplaceAll(s, ")", "")
	s = strings.ReplaceAll(s, "[", "")
	s = strings.ReplaceAll(s, "]", "")
	s = strings.ReplaceAll(s, ":", "")
	s = strings.ReplaceAll(s, "/", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

func matchTracksByName(currentTracks, altTracks []map[string]interface{}) []map[string]interface{} {
	matched := make([]map[string]interface{}, len(currentTracks))

	for i, currentTrack := range currentTracks {
		currentTitle, _ := currentTrack["title"].(string)
		currentTitle = strings.TrimSpace(strings.ToLower(currentTitle))

		bestMatch := -1
		bestScore := 0.0

		for j, altTrack := range altTracks {
			altTitle, _ := altTrack["title"].(string)
			altTitle = strings.TrimSpace(strings.ToLower(altTitle))

			if currentTitle == altTitle {
				bestMatch = j
				bestScore = 1.0
				break
			}

			score := stringSimilarity(currentTitle, altTitle)
			if score > bestScore && score >= 0.7 {
				bestScore = score
				bestMatch = j
			}
		}

		if bestMatch >= 0 {
			altTrack := altTracks[bestMatch]
			duration, _ := altTrack["duration"].(int)
			if duration > 0 {
				matched[i] = map[string]interface{}{
					"track_number": currentTrack["track_number"],
					"disc_number":  currentTrack["disc_number"],
					"position":     currentTrack["position"],
					"title":        currentTrack["title"],
					"duration":     duration,
				}
				continue
			}
		}

		matched[i] = currentTrack
	}

	return matched
}

func removeZeroWidthChars(s string) string {
	result := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= 0x200B && r <= 0x200F {
			continue
		}
		if r == 0xFEFF {
			continue
		}
		result = append(result, r)
	}
	return string(result)
}

func stringSimilarity(a, b string) float64 {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))

	if a == b {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}

	distance := levenshteinDistance(a, b)
	maxLen := max(len(a), len(b))

	return 1.0 - (float64(distance) / float64(maxLen))
}

func levenshteinDistance(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	matrix := make([][]int, len(a)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(b)+1)
		matrix[i][0] = i
	}
	for j := range matrix[0] {
		matrix[0][j] = j
	}

	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,
				matrix[i][j-1]+1,
				matrix[i-1][j-1]+cost,
			)
		}
	}

	return matrix[len(a)][len(b)]
}

func percentEncode(s string) string {
	return url.QueryEscape(s)
}

func percentEncodeValue(s string) string {
	s = strings.ReplaceAll(s, " ", "%20")
	s = strings.ReplaceAll(s, "!", "%21")
	s = strings.ReplaceAll(s, "#", "%23")
	s = strings.ReplaceAll(s, "$", "%24")
	s = strings.ReplaceAll(s, "&", "%26")
	s = strings.ReplaceAll(s, "'", "%27")
	s = strings.ReplaceAll(s, "(", "%28")
	s = strings.ReplaceAll(s, ")", "%29")
	s = strings.ReplaceAll(s, "*", "%2A")
	s = strings.ReplaceAll(s, "+", "%2B")
	s = strings.ReplaceAll(s, ",", "%2C")
	s = strings.ReplaceAll(s, "/", "%2F")
	s = strings.ReplaceAll(s, ":", "%3A")
	s = strings.ReplaceAll(s, ";", "%3B")
	s = strings.ReplaceAll(s, "=", "%3D")
	s = strings.ReplaceAll(s, "?", "%3F")
	s = strings.ReplaceAll(s, "@", "%40")
	s = strings.ReplaceAll(s, "[", "%5B")
	s = strings.ReplaceAll(s, "]", "%5D")
	return s
}
