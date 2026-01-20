package controllers

import (
	"io"
	"net/http"
	"strings"

	"vinylfo/config"
)

// downloadImage downloads an image from the given URL and returns the data, content type, and any error
func downloadImage(imageURL string) ([]byte, string, error) {
	if imageURL == "" {
		return nil, "", nil
	}

	client := config.DefaultClient()

	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return nil, "", err
	}

	req.Header.Set("User-Agent", "Vinylfo/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", err
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	if !strings.HasPrefix(contentType, "image/") {
		return nil, "", err
	}

	return data, contentType, nil
}
