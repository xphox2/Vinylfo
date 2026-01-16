package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDownloadImage_FixesImageDownload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/image.jpg" {
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	data, contentType, err := downloadImage(server.URL + "/image.jpg")

	if err != nil {
		t.Errorf("downloadImage failed: %v", err)
		return
	}

	if len(data) == 0 {
		t.Errorf("downloadImage returned empty data")
		return
	}

	if contentType != "image/jpeg" {
		t.Errorf("expected content type image/jpeg, got %s", contentType)
	}

	t.Logf("SUCCESS: Downloaded image - size=%d bytes, type=%s", len(data), contentType)
}

func TestDownloadImage_EmptyURL(t *testing.T) {
	data, contentType, err := downloadImage("")

	if err != nil {
		t.Errorf("downloadImage with empty URL should not fail, got: %v", err)
		return
	}

	if data != nil {
		t.Errorf("downloadImage with empty URL should return nil data")
	}

	if contentType != "" {
		t.Errorf("downloadImage with empty URL should return empty content type")
	}

	t.Logf("SUCCESS: Empty URL handled correctly")
}
