package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

// Common HTTP status codes and strings
const (
	StrStarted     = "started"
	StrOK          = "ok"
	StrNothingDone = "Nothing Done"
	StrData        = "data"
	StrTotal       = "total"
	StrError       = "error"
	StrJobLower    = "job"
	StrID          = "id"
	StrLimit       = "limit"
	StrPage        = "page"
	StrOrder       = "order"
	StrName        = "name"
	StrApikey      = "apikey"
)

// PaginationParams holds common pagination parameters
type PaginationParams struct {
	Limit  uint
	Offset uint
	Page   int
	Order  string
}

// StandardResponse represents a standard API response structure
type StandardResponse struct {
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
	Total int    `json:"total,omitempty"`
}

// parsePaginationParams extracts pagination parameters from request
func parsePaginationParams(ctx *gin.Context) PaginationParams {
	var params PaginationParams
	var limit, page int

	if queryParam, ok := ctx.GetQuery(StrLimit); ok && queryParam != "" {
		limit, _ = strconv.Atoi(queryParam)
		params.Limit = uint(limit)
	}

	if limit != 0 {
		if queryParam, ok := ctx.GetQuery(StrPage); ok && queryParam != "" {
			page, _ = strconv.Atoi(queryParam)
			params.Page = page
			if page >= 2 {
				params.Offset = uint((page - 1) * limit)
			}
		}
	}

	if queryParam, ok := ctx.GetQuery(StrOrder); ok && queryParam != "" {
		params.Order = queryParam
	}

	return params
}

// buildQuery creates a database query with pagination
func buildQuery(params PaginationParams) database.Querywithargs {
	query := database.Querywithargs{}
	if params.Limit > 0 {
		query.Limit = params.Limit
		query.Offset = int(params.Offset)
	}
	if params.Order != "" {
		query.OrderBy = params.Order
	}
	return query
}

// buildQueryWithWhere creates a database query with pagination and where clause
func buildQueryWithWhere(params PaginationParams, where string) database.Querywithargs {
	query := buildQuery(params)
	query.Where = where
	return query
}

// sendJSONResponse sends a standardized JSON response
func sendJSONResponse(ctx *gin.Context, status int, data any, total ...int) {
	response := StandardResponse{Data: data}
	if len(total) > 0 {
		response.Total = total[0]
	}
	ctx.JSON(status, response)
}

// sendJSONError sends a standardized JSON error response
func sendJSONError(ctx *gin.Context, status int, message string) {
	ctx.JSON(status, StandardResponse{Error: message})
}

// sendSuccess sends a standardized success response
func sendSuccess(ctx *gin.Context, message string) {
	sendJSONResponse(ctx, http.StatusOK, message)
}

// sendNotFound sends a standardized 404 response
func sendNotFound(ctx *gin.Context, message string) {
	sendJSONError(ctx, http.StatusNotFound, message)
}

// sendBadRequest sends a standardized 400 response
func sendBadRequest(ctx *gin.Context, message string) {
	sendJSONError(ctx, http.StatusBadRequest, message)
}

// sendUnauthorized sends a standardized 401 response
func sendUnauthorized(ctx *gin.Context, message string) {
	sendJSONError(ctx, http.StatusUnauthorized, message)
}

// sendForbidden sends a standardized 403 response
func sendForbidden(ctx *gin.Context, message string) {
	sendJSONError(ctx, http.StatusForbidden, message)
}

// sendInternalError sends a standardized 500 response
func sendInternalError(ctx *gin.Context, message string) {
	sendJSONError(ctx, http.StatusInternalServerError, message)
}

// handleDBError handles database operation errors consistently
func handleDBError(ctx *gin.Context, err error, successMessage string) {
	if err != nil {
		sendForbidden(ctx, err.Error())
	} else {
		sendSuccess(ctx, successMessage)
	}
}

// handleDBResultWithRows handles database operations that return affected rows
func handleDBResultWithRows(ctx *gin.Context, result any, err error) {
	if err != nil {
		sendForbidden(ctx, err.Error())
	} else {
		sendJSONResponse(ctx, http.StatusOK, result)
	}
}

// validateJobParam validates if a job parameter is in the allowed list
func validateJobParam(job string, allowedJobs string) bool {
	allowedList := make(map[string]bool)
	for _, allowedJob := range splitAndTrim(allowedJobs, ",") {
		allowedList[allowedJob] = true
	}
	return allowedList[job]
}

// splitAndTrim splits a string by delimiter and trims whitespace
func splitAndTrim(s, delimiter string) []string {
	parts := make([]string, 0)
	for _, part := range strings.Split(s, delimiter) {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

// getParamID extracts and validates an ID parameter
func getParamID(ctx *gin.Context, paramName string) (string, bool) {
	id := ctx.Param(paramName)
	if id == "" {
		sendBadRequest(ctx, "Invalid or missing "+paramName)
		return "", false
	}
	return id, true
}

// bindJSONWithValidation binds JSON request body with error handling
func bindJSONWithValidation(ctx *gin.Context, obj any) bool {
	if err := ctx.ShouldBindJSON(obj); err != nil {
		sendBadRequest(ctx, err.Error())
		return false
	}
	return true
}

// handleDBInsertOrUpdate handles database insert/update operations with consistent response
func handleDBInsertOrUpdate(ctx *gin.Context, result sql.Result, err error, isInsert bool) {
	if err != nil {
		sendForbidden(ctx, err.Error())
		return
	}

	var rows int64
	if isInsert {
		rows, _ = result.LastInsertId()
	} else {
		rows, _ = result.RowsAffected()
	}
	sendJSONResponse(ctx, http.StatusOK, rows)
}

// getCSRFToken extracts CSRF token from gin context if available
func getCSRFToken(c *gin.Context) string {
	if token, exists := c.Get("csrf_token"); exists {
		return token.(string)
	}
	return ""
}
