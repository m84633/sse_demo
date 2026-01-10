package dto

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type StatusResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
