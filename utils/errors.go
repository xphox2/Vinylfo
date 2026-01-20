package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Details string `json:"details,omitempty"`
}

func Success(ctx *gin.Context, statusCode int, data interface{}) {
	ctx.JSON(statusCode, data)
}

func Created(ctx *gin.Context, data interface{}) {
	ctx.JSON(http.StatusCreated, data)
}

func NoContent(ctx *gin.Context) {
	ctx.Status(http.StatusNoContent)
}

func Error(ctx *gin.Context, statusCode int, message string) {
	ctx.JSON(statusCode, ErrorResponse{
		Error: message,
		Code:  statusCode,
	})
}

func BadRequest(ctx *gin.Context, message string) {
	Error(ctx, http.StatusBadRequest, message)
}

func Unauthorized(ctx *gin.Context, message string) {
	Error(ctx, http.StatusUnauthorized, message)
}

func Forbidden(ctx *gin.Context, message string) {
	Error(ctx, http.StatusForbidden, message)
}

func NotFound(ctx *gin.Context, message string) {
	Error(ctx, http.StatusNotFound, message)
}

func Conflict(ctx *gin.Context, message string) {
	Error(ctx, http.StatusConflict, message)
}

func UnprocessableEntity(ctx *gin.Context, message string) {
	Error(ctx, http.StatusUnprocessableEntity, message)
}

func InternalError(ctx *gin.Context, message string) {
	Error(ctx, http.StatusInternalServerError, message)
}

func ServiceUnavailable(ctx *gin.Context, message string) {
	Error(ctx, http.StatusServiceUnavailable, message)
}

func ErrorWithDetails(ctx *gin.Context, statusCode int, message, details string) {
	ctx.JSON(statusCode, ErrorResponse{
		Error:   message,
		Code:    statusCode,
		Details: details,
	})
}

func ValidationError(ctx *gin.Context, message string) {
	Error(ctx, http.StatusBadRequest, "Validation error: "+message)
}
