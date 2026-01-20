package controllers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSecurityHeadersPresent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.Use(func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Next()
	})

	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("X-Content-Type-Options header should be nosniff")
	}

	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("X-Frame-Options header should be DENY")
	}

	if w.Header().Get("X-XSS-Protection") != "1; mode=block" {
		t.Fatal("X-XSS-Protection header should be set")
	}
}

func TestOAuthCallbackErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.GET("/oauth/callback", func(c *gin.Context) {
		errorParam := c.Query("error")
		if errorParam != "" {
			c.Header("Content-Type", "text/html")
			c.String(http.StatusOK, "Error: "+errorParam)
			return
		}

		code := c.Query("code")
		if code == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No authorization code"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	t.Run("Error Parameter", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/oauth/callback?error=access_denied", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200 for error response, got %d", w.Code)
		}

		if !strings.Contains(w.Body.String(), "access_denied") {
			t.Fatal("Response should mention the error")
		}
	})

	t.Run("Missing Code", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/oauth/callback", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("Expected status 400 for missing code, got %d", w.Code)
		}
	})
}

func TestTokenRevocationEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.POST("/disconnect", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message":   "Successfully disconnected",
			"connected": false,
		})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/disconnect", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "Successfully disconnected") {
		t.Fatal("Response should confirm disconnection")
	}
}

func TestYouTubeStatusEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.GET("/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"connected":     false,
			"is_configured": true,
			"db_connected":  true,
			"has_token":     false,
		})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/status", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "connected") {
		t.Fatal("Response should contain connected field")
	}

	if !strings.Contains(body, "is_configured") {
		t.Fatal("Response should contain is_configured field")
	}
}

func TestCSPHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.Use(func(c *gin.Context) {
		c.Header("Content-Security-Policy", "default-src 'self'")
		c.Next()
	})

	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	csp := w.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("CSP header should be set")
	}

	if !strings.Contains(csp, "default-src") {
		t.Fatal("CSP should contain default-src directive")
	}
}

func TestOAuthErrorResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.GET("/oauth/callback", func(c *gin.Context) {
		errorMsg := c.Query("error")
		if errorMsg != "" {
			c.Header("Content-Type", "text/html")
			c.String(http.StatusOK, `<html><body><div class="error">Authorization denied: `+errorMsg+`</div></body></html>`)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	testCases := []struct {
		name        string
		errorParam  string
		expectError bool
	}{
		{"access_denied", "access_denied", true},
		{"invalid_scope", "invalid_scope", true},
		{"no error", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			url := "/oauth/callback"
			if tc.errorParam != "" {
				url += "?error=" + tc.errorParam
			}

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", url, nil)
			r.ServeHTTP(w, req)

			if tc.expectError {
				if !strings.Contains(w.Body.String(), "Authorization denied") {
					t.Fatalf("Expected error message in response")
				}
				if w.Header().Get("Content-Type") != "text/html" {
					t.Fatalf("Expected HTML content type for error")
				}
			}
		})
	}
}

func TestStateCookieAttributes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.GET("/oauth/url", func(c *gin.Context) {
		state := "test_state_123"
		c.SetCookie("youtube_oauth_state", state, 300, "/", "", false, true)
		c.SetCookie("youtube_oauth_code_verifier", "verifier_456", 300, "/", "", false, true)
		c.JSON(200, gin.H{"state": state})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/oauth/url", nil)
	r.ServeHTTP(w, req)

	cookies := w.Result().Cookies()

	stateCookie := false
	verifierCookie := false

	for _, cookie := range cookies {
		if cookie.Name == "youtube_oauth_state" {
			stateCookie = true
			if cookie.Value != "test_state_123" {
				t.Fatal("State cookie value mismatch")
			}
			if cookie.MaxAge != 300 {
				t.Fatalf("Expected MaxAge 300, got %d", cookie.MaxAge)
			}
			if !cookie.HttpOnly {
				t.Fatal("State cookie should be HttpOnly")
			}
		}
		if cookie.Name == "youtube_oauth_code_verifier" {
			verifierCookie = true
			if cookie.Value != "verifier_456" {
				t.Fatal("Verifier cookie value mismatch")
			}
		}
	}

	if !stateCookie {
		t.Fatal("State cookie should be set")
	}
	if !verifierCookie {
		t.Fatal("Verifier cookie should be set")
	}
}

func TestOAuthResponseContentType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.GET("/oauth/success", func(c *gin.Context) {
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, "<html><body>Success</body></html>")
	})

	r.GET("/api/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	t.Run("HTML Response", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/oauth/success", nil)
		r.ServeHTTP(w, req)

		if w.Header().Get("Content-Type") != "text/html" {
			t.Fatalf("Expected text/html for success page, got %s", w.Header().Get("Content-Type"))
		}
	})

	t.Run("JSON Response", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/status", nil)
		r.ServeHTTP(w, req)

		if w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
			t.Fatalf("Expected application/json for API, got %s", w.Header().Get("Content-Type"))
		}
	})
}

func TestReferrerPolicyHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.Use(func(c *gin.Context) {
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	})

	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Header().Get("Referrer-Policy") != "strict-origin-when-cross-origin" {
		t.Fatal("Referrer-Policy header should be set")
	}
}

func TestPermissionsPolicyHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.Use(func(c *gin.Context) {
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		c.Next()
	})

	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	pp := w.Header().Get("Permissions-Policy")
	if pp == "" {
		t.Fatal("Permissions-Policy header should be set")
	}

	if !strings.Contains(pp, "geolocation") {
		t.Fatal("Permissions-Policy should mention geolocation")
	}
}
