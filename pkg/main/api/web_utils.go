package api

import (
	"net/http"

	gin "github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// HTML generation helper functions
func formInput(inputType, name, id, class, value string, attrs ...gomponents.Node) gomponents.Node {
	allAttrs := []gomponents.Node{
		html.Type(inputType),
		html.Name(name),
		html.ID(id),
		html.Class(class),
	}
	if value != "" {
		allAttrs = append(allAttrs, html.Value(value))
	}
	allAttrs = append(allAttrs, attrs...)
	return html.Input(allAttrs...)
}

func formTextInput(name, id, value string, attrs ...gomponents.Node) gomponents.Node {
	return formInput("text", name, id, "form-control-modern", value, attrs...)
}

func createOption(value, text string, selected bool) gomponents.Node {
	attrs := []gomponents.Node{
		html.Value(value),
		gomponents.Text(text),
	}
	if selected {
		attrs = append(attrs, html.Selected())
	}
	return html.Option(attrs...)
}

func formCheckboxInput(name, id string, checked bool, attrs ...gomponents.Node) gomponents.Node {
	allAttrs := []gomponents.Node{
		html.Type("checkbox"),
		html.Name(name),
		html.ID(id),
		html.Class("form-check-input-modern"),
	}
	if checked {
		allAttrs = append(allAttrs, html.Checked())
	}
	allAttrs = append(allAttrs, attrs...)
	return html.Input(allAttrs...)
}

// Response helper functions
func sendDataTablesResponse(ctx *gin.Context, total, final int, data any) {
	ctx.JSON(http.StatusOK, gin.H{
		"sEcho":                getParamValue(ctx, "sEcho"),
		"iTotalRecords":        total,
		"iTotalDisplayRecords": final,
		"aaData":               data,
	})
}

func sendErrorResponse(ctx *gin.Context, statusCode int, message string) {
	ctx.JSON(statusCode, gin.H{
		"success": false,
		"error":   message,
	})
}

func sendOperationResult(ctx *gin.Context, err error) {
	response := gin.H{
		"success": err == nil,
	}
	if err != nil {
		response["error"] = err.Error()
	} else {
		response["error"] = ""
	}
	ctx.JSON(http.StatusOK, response)
}

func sendSelect2Response(ctx *gin.Context, results []map[string]any, hasMore bool) {
	ctx.JSON(http.StatusOK, gin.H{
		"results": results,
		"pagination": gin.H{
			"more": hasMore,
		},
	})
}

func createSelect2Option(id any, text string) map[string]any {
	return map[string]any{
		"id":   id,
		"text": text,
	}
}

func createSelect2OptionPtr(id any, text string) *map[string]any {
	option := map[string]any{
		"id":   id,
		"text": text,
	}
	return &option
}

func createSelect2OptionString(value string, text string) map[string]any {
	return map[string]any{
		"id":   value,
		"text": text,
	}
}

// getParam retrieves a parameter from either GET query or POST form data
func getParam(ctx *gin.Context, key, defaultValue string) string {
	if ctx.Request.Method == "POST" {
		return ctx.DefaultPostForm(key, defaultValue)
	}
	return ctx.DefaultQuery(key, defaultValue)
}

// getParamValue retrieves a parameter from either GET query or POST form data (no default)
func getParamValue(ctx *gin.Context, key string) string {
	if ctx.Request.Method == "POST" {
		return ctx.PostForm(key)
	}
	return ctx.Query(key)
}
