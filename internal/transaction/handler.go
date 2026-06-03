package transaction

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /transfers", h.CreateTransfer)
	mux.HandleFunc("GET /transfers/{transaction_id}", h.GetTransfer)
}

func (h *Handler) CreateTransfer(w http.ResponseWriter, r *http.Request) {
	var req CreateTransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}

	resp, err := h.service.CreateTransfer(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrInvalidRequest) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		slog.Error("failed to create transfer", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create transfer"})
		return
	}

	writeJSON(w, http.StatusAccepted, resp)
}

func (h *Handler) GetTransfer(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("transaction_id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "transaction_id must be a valid uuid"})
		return
	}

	tx, err := h.service.GetTransfer(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrTransactionNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "transaction not found"})
			return
		}
		slog.Error("failed to get transfer", "transaction_id", id, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get transfer"})
		return
	}

	writeJSON(w, http.StatusOK, tx)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}
