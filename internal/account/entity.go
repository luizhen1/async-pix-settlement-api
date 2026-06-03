package account

import (
	"time"

	"github.com/google/uuid"
)

type Account struct {
	ID           uuid.UUID `json:"id"`
	OwnerName    string    `json:"owner_name"`
	BalanceCents int64     `json:"balance_cents"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	UpdatedAt    time.Time `json:"updated_at,omitempty"`
}
