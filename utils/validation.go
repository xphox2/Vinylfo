package utils

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type ValidationErr struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationResult struct {
	Valid  bool
	Errors []ValidationErr
}

func (v *ValidationResult) AddError(field, message string) {
	v.Valid = false
	v.Errors = append(v.Errors, ValidationErr{
		Field:   field,
		Message: message,
	})
}

func (v *ValidationResult) HasErrors() bool {
	return !v.Valid
}

func (v *ValidationResult) Error() string {
	if !v.Valid {
		messages := make([]string, len(v.Errors))
		for i, e := range v.Errors {
			messages[i] = e.Message
		}
		return strings.Join(messages, "; ")
	}
	return ""
}

func NewValidationResult() *ValidationResult {
	return &ValidationResult{Valid: true}
}

func ValidateRequired(value interface{}, fieldName string) *ValidationResult {
	result := NewValidationResult()
	if value == nil {
		result.AddError(fieldName, fieldName+" is required")
		return result
	}

	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			result.AddError(fieldName, fieldName+" is required")
		}
	case int:
		if v == 0 {
			result.AddError(fieldName, fieldName+" is required")
		}
	case uint:
		if v == 0 {
			result.AddError(fieldName, fieldName+" is required")
		}
	case float64:
		if v == 0 {
			result.AddError(fieldName, fieldName+" is required")
		}
	}

	return result
}

func ValidatePositiveInt(value int, fieldName string) *ValidationResult {
	result := NewValidationResult()
	if value <= 0 {
		result.AddError(fieldName, fieldName+" must be a positive number")
	}
	return result
}

func ValidateNonNegativeInt(value int, fieldName string) *ValidationResult {
	result := NewValidationResult()
	if value < 0 {
		result.AddError(fieldName, fieldName+" cannot be negative")
	}
	return result
}

func ValidateUintID(value uint, fieldName string) *ValidationResult {
	result := NewValidationResult()
	if value == 0 {
		result.AddError(fieldName, "valid "+fieldName+" is required")
	}
	return result
}

func ValidateStringLength(value, fieldName string, min, max int) *ValidationResult {
	result := NewValidationResult()
	length := len(strings.TrimSpace(value))
	if length < min {
		result.AddError(fieldName, fieldName+" must be at least "+strconv.Itoa(min)+" characters")
	}
	if max > 0 && length > max {
		result.AddError(fieldName, fieldName+" must be at most "+strconv.Itoa(max)+" characters")
	}
	return result
}

func ValidateStringNotEmpty(value, fieldName string) *ValidationResult {
	result := NewValidationResult()
	if strings.TrimSpace(value) == "" {
		result.AddError(fieldName, fieldName+" cannot be empty")
	}
	return result
}

var httpURLRegex = regexp.MustCompile(`^https?://`)

func ValidateURL(value, fieldName string) *ValidationResult {
	result := NewValidationResult()
	if value == "" {
		return result
	}

	if !httpURLRegex.MatchString(value) {
		result.AddError(fieldName, fieldName+" must be a valid URL (http:// or https://)")
		return result
	}

	_, err := url.ParseRequestURI(value)
	if err != nil {
		result.AddError(fieldName, fieldName+" must be a valid URL")
	}

	return result
}

func ValidateEnum(value, fieldName string, allowedValues []string) *ValidationResult {
	result := NewValidationResult()
	if value == "" {
		return result
	}

	for _, allowed := range allowedValues {
		if value == allowed {
			return result
		}
	}

	result.AddError(fieldName, fieldName+" must be one of: "+strings.Join(allowedValues, ", "))
	return result
}

func ValidateDuration(value int, fieldName string) *ValidationResult {
	result := NewValidationResult()
	if value < 0 {
		result.AddError(fieldName, fieldName+" cannot be negative")
	}
	if value > 86400 { // 24 hours in seconds
		result.AddError(fieldName, fieldName+" exceeds maximum (24 hours)")
	}
	return result
}

func ValidatePlaylistID(value string) *ValidationResult {
	result := NewValidationResult()
	if strings.TrimSpace(value) == "" {
		result.AddError("playlist_id", "playlist_id is required")
		return result
	}
	if len(value) > 255 {
		result.AddError("playlist_id", "playlist_id must be at most 255 characters")
	}
	return result
}

func ValidateDiscogsID(value int) *ValidationResult {
	result := NewValidationResult()
	if value <= 0 {
		result.AddError("discogs_id", "valid discogs_id is required")
	}
	return result
}

func ValidateYear(value int) *ValidationResult {
	result := NewValidationResult()
	if value > 0 {
		currentYear := 2026
		if value < 1900 || value > currentYear+5 {
			result.AddError("release_year", "release_year must be between 1900 and "+strconv.Itoa(currentYear+5))
		}
	}
	return result
}

func ValidatePageParams(page, limit string) *ValidationResult {
	result := NewValidationResult()

	if page != "" {
		p, err := strconv.Atoi(page)
		if err != nil || p < 1 {
			result.AddError("page", "page must be a positive integer")
		}
	}

	if limit != "" {
		l, err := strconv.Atoi(limit)
		if err != nil || l < 1 {
			result.AddError("limit", "limit must be a positive integer")
		} else if l > 100 {
			result.AddError("limit", "limit must be at most 100")
		}
	}

	return result
}

func ValidateRequest(ctx *gin.Context, validators ...*ValidationResult) bool {
	for _, v := range validators {
		if v.HasErrors() {
			BadRequest(ctx, v.Error())
			return false
		}
	}
	return true
}

func BindAndValidate(ctx *gin.Context, dest interface{}, validators ...*ValidationResult) bool {
	if err := ctx.ShouldBindJSON(dest); err != nil {
		SendValidationError(ctx, err.Error())
		return false
	}

	for _, v := range validators {
		if v.HasErrors() {
			SendValidationError(ctx, v.Error())
			return false
		}
	}

	return true
}

func SendValidationError(ctx *gin.Context, message string) {
	ctx.JSON(http.StatusBadRequest, gin.H{
		"error":   "Validation error",
		"code":    http.StatusBadRequest,
		"details": message,
	})
}
