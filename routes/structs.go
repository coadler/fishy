package routes

//
var secrets SecretStrings

// APIResponse is a standard API response.
type APIResponse struct {
	Error   bool        `json:"error"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

//
type SecretStrings struct {
	InvEE []string `json:"invee"`
}
