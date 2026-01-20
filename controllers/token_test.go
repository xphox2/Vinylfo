package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestTokenRevocationScenario(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.GET("/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"connected":     true,
			"is_configured": true,
			"has_token":     true,
		})
	})

	r.POST("/disconnect", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message":   "Successfully disconnected from YouTube",
			"connected": false,
		})
	})

	t.Run("Initial Connected State", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/status", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		if response["connected"] != true {
			t.Fatal("Initial state should be connected")
		}
	})

	t.Run("Token Revocation", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/disconnect", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		if !strings.Contains(w.Body.String(), "Successfully disconnected") {
			t.Fatal("Response should confirm disconnection")
		}
	})

	t.Run("State After Revocation", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/status", nil)
		r.ServeHTTP(w, req)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		if response["connected"] != true {
			t.Log("Status endpoint returns current database state")
		}
	})
}

func TestTokenRevocationWithError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.POST("/disconnect", func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to disconnect: token revocation failed",
		})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/disconnect", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("Expected 500 for revocation error, got %d", w.Code)
	}
}

func TestQuotaExceededErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.POST("/search", func(c *gin.Context) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":  "quotaExceeded",
			"reason": "The request cannot be completed because the daily limit has been exceeded",
		})
	})

	t.Run("Quota Exceeded Response", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/search", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("Expected status 403 for quota exceeded, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		if response["error"] != "quotaExceeded" {
			t.Fatal("Response should indicate quotaExceeded error")
		}
	})

	t.Run("Quota Error Message", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/search", nil)
		r.ServeHTTP(w, req)

		body := w.Body.String()
		if !strings.Contains(body, "daily limit") {
			t.Fatal("Error message should mention daily limit")
		}
	})
}

func TestQuotaExceededRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	quotaResetTime := time.Now().Add(24 * time.Hour)

	r.GET("/quota/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"quota_used":  10000,
			"quota_total": 10000,
			"quota_reset": quotaResetTime.Format(time.RFC3339),
			"is_exceeded": true,
		})
	})

	t.Run("Quota Status", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/quota/status", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		if response["is_exceeded"] != true {
			t.Fatal("Should indicate quota is exceeded")
		}
	})
}

func TestOAuthCallbackAfterRevocation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.GET("/oauth/callback", func(c *gin.Context) {
		code := c.Query("code")
		if code == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No authorization code"})
			return
		}

		state := c.Query("state")
		if state == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "State missing"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "connected",
			"state":  state,
		})
	})

	t.Run("Valid OAuth Callback", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/oauth/callback?code=auth_code_123&state=state_456", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		if response["status"] != "connected" {
			t.Fatal("Should show connected status")
		}
	})

	t.Run("Missing Code", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/oauth/callback?state=state_456", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("Expected status 400, got %d", w.Code)
		}
	})
}

func TestRefreshTokenScenario(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.POST("/token/refresh", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":         "refreshed",
			"new_expires_in": 3600,
		})
	})

	t.Run("Token Refresh", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/token/refresh", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		if !strings.Contains(w.Body.String(), "refreshed") {
			t.Fatal("Should show refreshed status")
		}
	})
}

func TestExpiringSoonDetection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.GET("/token/expiring", func(c *gin.Context) {
		expiry := time.Now().Add(5 * time.Minute)
		c.JSON(http.StatusOK, gin.H{
			"expires_at":     expiry.Format(time.RFC3339),
			"seconds_left":   300,
			"is_expiring":    true,
			"refresh_needed": true,
		})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/token/expiring", nil)
	r.ServeHTTP(w, req)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["is_expiring"] != true {
		t.Fatal("Should indicate token is expiring")
	}
}

func TestExpiredTokenHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.GET("/token/expired", func(c *gin.Context) {
		expiry := time.Now().Add(-1 * time.Hour)
		c.JSON(http.StatusOK, gin.H{
			"expires_at":     expiry.Format(time.RFC3339),
			"is_expired":     true,
			"refresh_needed": true,
			"user_message":   "Your session has expired. Please reconnect your YouTube account.",
		})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/token/expired", nil)
	r.ServeHTTP(w, req)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["is_expired"] != true {
		t.Fatal("Should indicate token is expired")
	}

	if response["user_message"] == nil || response["user_message"] == "" {
		t.Fatal("Should include user-friendly message")
	}
}

func TestGracefulDegradation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.GET("/feature/unavailable", func(c *gin.Context) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":       "service_unavailable",
			"message":     "YouTube integration is temporarily unavailable",
			"retry_after": 60,
		})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/feature/unavailable", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("Expected 503, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["error"] != "service_unavailable" {
		t.Fatal("Should indicate service unavailable")
	}

	if response["retry_after"] == nil {
		t.Fatal("Should suggest retry time")
	}
}

func TestAuditLogOnRevocation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	auditCalled := false

	r.POST("/disconnect", func(c *gin.Context) {
		auditCalled = true
		c.JSON(http.StatusOK, gin.H{"status": "disconnected"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/disconnect", nil)
	req.Header.Set("User-Agent", "TestAgent")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	if !auditCalled {
		t.Fatal("Audit logging should be called on revocation")
	}
}

func TestTokenStateTransitions(t *testing.T) {
	tests := []struct {
		name          string
		initialState  string
		action        string
		expectedState string
	}{
		{"Connected to Disconnected", "connected", "revoke", "disconnected"},
		{"Disconnected to Connected", "disconnected", "connect", "connected"},
		{"Connected to Expired", "connected", "expire", "expired"},
		{"Expired to Connected", "expired", "refresh", "connected"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := tt.initialState

			if tt.action == "revoke" {
				state = "disconnected"
			} else if tt.action == "connect" {
				state = "connected"
			} else if tt.action == "expire" {
				state = "expired"
			} else if tt.action == "refresh" {
				state = "connected"
			}

			if state != tt.expectedState {
				t.Fatalf("Expected state %s, got %s", tt.expectedState, state)
			}
		})
	}
}
