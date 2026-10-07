package httpx

import (
	"encoding/json/v2"
	"net/http"
)

type ErrorResponse struct {
	Message string `json:"message"`
}

func WriteOKResponse(w http.ResponseWriter, v any) {
	WriteResponse(w, http.StatusOK, v)
}

func WriteCreatedResponse(w http.ResponseWriter, v any) {
	WriteResponse(w, http.StatusCreated, v)
}

func WriteFailureInjection(w http.ResponseWriter, statusCode int, reason string) {
	if statusCode < 100 || statusCode > 599 {
		statusCode = http.StatusInternalServerError
	}

	WriteResponse(w, statusCode, map[string]string{
		"reason": reason,
	})
}

func WriteEmptyResponse(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func WriteResponse(w http.ResponseWriter, status int, v any) {
	if v == nil {
		w.WriteHeader(status)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	body, err := json.Marshal(v)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"Internal Server Error"}`))
		return
	}

	w.WriteHeader(status)
	_, _ = w.Write(body)
}
