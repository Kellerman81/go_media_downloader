package api

// Response shapes referenced only by the swag annotations (@Success/@Failure)
// on the REST handlers. Handlers build these bodies via sendJSONResponse /
// sendSuccess / sendJSONError; the types exist so `swag init` can resolve them.

// Jsondata is the generic success body.
type Jsondata struct {
	Data any `json:"data"`
}

// Jsondataerror is a body carrying both data and an error message.
type Jsondataerror struct {
	Data  any    `json:"data"`
	Error string `json:"error"`
}

// Jsondatarows is a paginated list body.
type Jsondatarows struct {
	Data  any `json:"data"`
	Total int `json:"total"`
}

// Jsonerror is the generic error body.
type Jsonerror struct {
	Error string `json:"error"`
}
