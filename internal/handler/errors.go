package handler

import "net/http"

// FE / 다른 API 와 맞춘 에러 JSON 형태.
type apiErrorBody struct {
	Message string   `json:"message"`
	Errors  []string `json:"errors"`
}

func WriteInvalidToken(w http.ResponseWriter) {
	writeAPIError(w, http.StatusUnauthorized, "로그인 정보가 유효하지 않습니다. 다시 로그인해 주세요.")
}

func writeAPIError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, apiErrorBody{Message: message, Errors: []string{}})
}
