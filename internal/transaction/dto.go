package transaction

import "github.com/google/uuid"

type CreateTransferRequest struct {
	TransactionID string  `json:"transaction_id,omitempty"`
	FromAccountID string  `json:"from_account_id"`
	ToAccountID   string  `json:"to_account_id"`
	Amount        float64 `json:"amount"`
}

type CreateTransferResponse struct {
	TransactionID uuid.UUID `json:"transaction_id"`
	Status        Status    `json:"status"`
	Message       string    `json:"message"`
}

type TransferEvent struct {
	TransactionID uuid.UUID `json:"transaction_id"`
	FromAccountID uuid.UUID `json:"from_account_id"`
	ToAccountID   uuid.UUID `json:"to_account_id"`
	Amount        float64   `json:"amount"`
}

type ProcessResult struct {
	Status  Status
	Message string
}
