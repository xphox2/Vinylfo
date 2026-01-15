package discogs

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

type OAuthConfigValidationError struct {
	Field   string
	Message string
}

func (e OAuthConfigValidationError) Error() string {
	return fmt.Sprintf("OAuth config error: %s - %s", e.Field, e.Message)
}

type OAuthConfigValidationResult struct {
	IsValid  bool
	Errors   []OAuthConfigValidationError
	Warnings []string
}

func ValidateOAuthConfig() *OAuthConfigValidationResult {
	result := &OAuthConfigValidationResult{
		IsValid:  true,
		Errors:   []OAuthConfigValidationError{},
		Warnings: []string{},
	}

	consumerKey := os.Getenv("DISCOGS_CONSUMER_KEY")
	consumerSecret := os.Getenv("DISCOGS_CONSUMER_SECRET")
	callbackURL := os.Getenv("DISCOGS_CALLBACK_URL")

	fmt.Println("=== Validating OAuth Configuration ===")

	if consumerKey == "" {
		result.IsValid = false
		result.Errors = append(result.Errors, OAuthConfigValidationError{
			Field:   "DISCOGS_CONSUMER_KEY",
			Message: "is not set",
		})
		fmt.Println("✗ DISCOGS_CONSUMER_KEY is not set")
	} else {
		fmt.Printf("✓ DISCOGS_CONSUMER_KEY is set: %s\n", maskValue(consumerKey))
	}

	if consumerSecret == "" {
		result.IsValid = false
		result.Errors = append(result.Errors, OAuthConfigValidationError{
			Field:   "DISCOGS_CONSUMER_SECRET",
			Message: "is not set",
		})
		fmt.Println("✗ DISCOGS_CONSUMER_SECRET is not set")
	} else {
		fmt.Printf("✓ DISCOGS_CONSUMER_SECRET is set: %s\n", maskValue(consumerSecret))
	}

	if callbackURL == "" {
		result.IsValid = false
		result.Errors = append(result.Errors, OAuthConfigValidationError{
			Field:   "DISCOGS_CALLBACK_URL",
			Message: "is not set",
		})
		fmt.Println("✗ DISCOGS_CALLBACK_URL is not set")
	} else {
		fmt.Printf("✓ DISCOGS_CALLBACK_URL is set: %s\n", callbackURL)
	}

	if callbackURL != "" {
		if !strings.Contains(callbackURL, "/api/discogs/oauth/callback") {
			result.IsValid = false
			result.Errors = append(result.Errors, OAuthConfigValidationError{
				Field:   "DISCOGS_CALLBACK_URL",
				Message: "does not contain expected path /api/discogs/oauth/callback",
			})
			fmt.Println("✗ DISCOGS_CALLBACK_URL does not contain expected path")
		}

		if strings.HasPrefix(callbackURL, "https://") {
			fmt.Println("⚠ DISCOGS_CALLBACK_URL uses HTTPS (verify Discogs app settings match)")
		} else if strings.HasPrefix(callbackURL, "http://") {
			if strings.Contains(callbackURL, "localhost") {
				fmt.Println("✓ DISCOGS_CALLBACK_URL uses HTTP with localhost (acceptable for development)")
			} else {
				result.Warnings = append(result.Warnings, "DISCOGS_CALLBACK_URL uses HTTP without localhost (should use HTTPS for production)")
				fmt.Println("⚠ DISCOGS_CALLBACK_URL uses HTTP without localhost")
			}
		} else {
			result.IsValid = false
			result.Errors = append(result.Errors, OAuthConfigValidationError{
				Field:   "DISCOGS_CALLBACK_URL",
				Message: "has invalid protocol (must start with http:// or https://)",
			})
			fmt.Println("✗ DISCOGS_CALLBACK_URL has invalid protocol")
		}

		if _, err := url.Parse(callbackURL); err != nil {
			result.IsValid = false
			result.Errors = append(result.Errors, OAuthConfigValidationError{
				Field:   "DISCOGS_CALLBACK_URL",
				Message: fmt.Sprintf("is not a valid URL: %v", err),
			})
			fmt.Printf("✗ DISCOGS_CALLBACK_URL is not a valid URL: %v\n", err)
		}
	}

	if len(result.Warnings) > 0 {
		fmt.Println("\n=== Warnings ===")
		for _, warning := range result.Warnings {
			fmt.Printf("⚠ %s\n", warning)
		}
	}

	if !result.IsValid {
		fmt.Println("\n=== Configuration Validation Failed ===")
		fmt.Println("Please fix the errors above before using OAuth.")
	}

	return result
}

func PrintOAuthConfigSummary() {
	fmt.Println("\n=== OAuth Configuration Summary ===")
	fmt.Println("Current OAuth Settings:")

	consumerKey := os.Getenv("DISCOGS_CONSUMER_KEY")
	consumerSecret := os.Getenv("DISCOGS_CONSUMER_SECRET")
	callbackURL := os.Getenv("DISCOGS_CALLBACK_URL")

	if consumerKey != "" {
		fmt.Printf("  DISCOGS_CONSUMER_KEY: %s\n", maskValue(consumerKey))
	} else {
		fmt.Println("  DISCOGS_CONSUMER_KEY: (not set)")
	}

	if consumerSecret != "" {
		fmt.Printf("  DISCOGS_CONSUMER_SECRET: %s\n", maskValue(consumerSecret))
	} else {
		fmt.Println("  DISCOGS_CONSUMER_SECRET: (not set)")
	}

	if callbackURL != "" {
		fmt.Printf("  DISCOGS_CALLBACK_URL: %s\n", callbackURL)
	} else {
		fmt.Println("  DISCOGS_CALLBACK_URL: (not set)")
	}
}
