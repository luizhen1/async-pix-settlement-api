package transaction

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

type CreateTransferRequest struct {
	TransactionID string `json:"transaction_id,omitempty"`
	FromAccountID string `json:"from_account_id"`
	ToAccountID   string `json:"to_account_id"`
	AmountCents   int64  `json:"amount_cents"`
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
	AmountCents   int64     `json:"amount_cents"`
}

type ProcessResult struct {
	Status  Status
	Message string
}

func (r *CreateTransferRequest) UnmarshalJSON(data []byte) error {
	type rawRequest struct {
		TransactionID string          `json:"transaction_id,omitempty"`
		FromAccountID string          `json:"from_account_id"`
		ToAccountID   string          `json:"to_account_id"`
		Amount        json.RawMessage `json:"amount"`
		AmountCents   *int64          `json:"amount_cents"`
	}

	var raw rawRequest
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	amountCents := int64(0)
	if raw.AmountCents != nil {
		amountCents = *raw.AmountCents
	} else if len(raw.Amount) > 0 && string(raw.Amount) != "null" {
		parsed, err := parseDecimalAmountToCents(raw.Amount)
		if err != nil {
			return err
		}
		amountCents = parsed
	}

	r.TransactionID = raw.TransactionID
	r.FromAccountID = raw.FromAccountID
	r.ToAccountID = raw.ToAccountID
	r.AmountCents = amountCents
	return nil
}

func parseDecimalAmountToCents(raw json.RawMessage) (int64, error) {
	value := strings.TrimSpace(string(raw))
	if value == "" {
		return 0, fmt.Errorf("%w: amount is required", ErrInvalidRequest)
	}
	if strings.HasPrefix(value, `"`) {
		if err := json.Unmarshal(raw, &value); err != nil {
			return 0, fmt.Errorf("%w: amount must be a decimal string or amount_cents integer", ErrInvalidRequest)
		}
		value = strings.TrimSpace(value)
	}

	if strings.HasPrefix(value, "-") {
		return 0, fmt.Errorf("%w: amount must be greater than zero", ErrInvalidRequest)
	}

	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, fmt.Errorf("%w: amount must have at most two decimal places", ErrInvalidRequest)
	}
	if !allDigits(parts[0]) {
		return 0, fmt.Errorf("%w: amount must contain only digits and decimal point", ErrInvalidRequest)
	}

	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
		if len(fraction) > 2 || !allDigits(fraction) {
			return 0, fmt.Errorf("%w: amount must have at most two decimal places", ErrInvalidRequest)
		}
	}
	for len(fraction) < 2 {
		fraction += "0"
	}

	reais, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: amount is too large", ErrInvalidRequest)
	}
	centavos, err := strconv.ParseInt(fraction, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: amount is invalid", ErrInvalidRequest)
	}
	if reais > (1<<63-1-centavos)/100 {
		return 0, fmt.Errorf("%w: amount is too large", ErrInvalidRequest)
	}

	return reais*100 + centavos, nil
}

func allDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return value != ""
}
