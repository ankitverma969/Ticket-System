package handlers

import (
	"net/http"

	"ticket-system/internal/response"
)

// HealthResponse represents the payload returned by GET /health.
type HealthResponse struct {
	Status string `json:"status"`
}

// Health handles the GET /health endpoint and returns {"status":"ok"}.
func Health(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, HealthResponse{
		Status: "ok",
	})
}
