package transaction

import (
	"encoding/json"
	"testing"
)

func TestCreateTransferRequestParsesAmountWithoutFloat(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantCents   int64
		wantFailure bool
	}{
		{
			name:      "amount cents",
			body:      `{"from_account_id":"11111111-1111-1111-1111-111111111111","to_account_id":"22222222-2222-2222-2222-222222222222","amount_cents":1050}`,
			wantCents: 1050,
		},
		{
			name:      "decimal string",
			body:      `{"from_account_id":"11111111-1111-1111-1111-111111111111","to_account_id":"22222222-2222-2222-2222-222222222222","amount":"10.50"}`,
			wantCents: 1050,
		},
		{
			name:      "decimal json number",
			body:      `{"from_account_id":"11111111-1111-1111-1111-111111111111","to_account_id":"22222222-2222-2222-2222-222222222222","amount":10.5}`,
			wantCents: 1050,
		},
		{
			name:        "too many decimal places",
			body:        `{"from_account_id":"11111111-1111-1111-1111-111111111111","to_account_id":"22222222-2222-2222-2222-222222222222","amount":"10.999"}`,
			wantFailure: true,
		},
		{
			name:        "scientific notation rejected",
			body:        `{"from_account_id":"11111111-1111-1111-1111-111111111111","to_account_id":"22222222-2222-2222-2222-222222222222","amount":1e3}`,
			wantFailure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req CreateTransferRequest
			err := json.Unmarshal([]byte(tt.body), &req)
			if tt.wantFailure {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if req.AmountCents != tt.wantCents {
				t.Fatalf("AmountCents = %d, want %d", req.AmountCents, tt.wantCents)
			}
		})
	}
}
