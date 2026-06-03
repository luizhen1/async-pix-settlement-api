package transaction

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"async-pix-settlement-api/internal/account"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidRequest      = errors.New("invalid request")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrDuplicateMessage    = errors.New("duplicate message")
)

type Producer interface {
	PublishTransfer(ctx context.Context, event TransferEvent) error
}

type Service struct {
	transactions Repository
	accounts     account.Repository
	producer     Producer
	db           *pgxpool.Pool
}

func NewService(transactions Repository, accounts account.Repository, producer Producer, db *pgxpool.Pool) *Service {
	return &Service{
		transactions: transactions,
		accounts:     accounts,
		producer:     producer,
		db:           db,
	}
}

func (s *Service) CreateTransfer(ctx context.Context, req CreateTransferRequest) (CreateTransferResponse, error) {
	fromID, err := parseRequiredUUID(req.FromAccountID, "from_account_id")
	if err != nil {
		return CreateTransferResponse{}, err
	}
	toID, err := parseRequiredUUID(req.ToAccountID, "to_account_id")
	if err != nil {
		return CreateTransferResponse{}, err
	}
	if fromID == toID {
		return CreateTransferResponse{}, fmt.Errorf("%w: from_account_id and to_account_id must be different", ErrInvalidRequest)
	}
	if req.Amount <= 0 {
		return CreateTransferResponse{}, fmt.Errorf("%w: amount must be greater than zero", ErrInvalidRequest)
	}

	txID := uuid.New()
	if strings.TrimSpace(req.TransactionID) != "" {
		txID, err = uuid.Parse(req.TransactionID)
		if err != nil {
			return CreateTransferResponse{}, fmt.Errorf("%w: transaction_id must be a valid uuid", ErrInvalidRequest)
		}
	}

	if err := s.ensureAccountsExist(ctx, fromID, toID); err != nil {
		return CreateTransferResponse{}, err
	}

	tx := Transaction{
		ID:            txID,
		FromAccountID: fromID,
		ToAccountID:   toID,
		Amount:        req.Amount,
		Status:        StatusProcessing,
	}
	if err := s.transactions.Create(ctx, tx); err != nil {
		if isUniqueViolation(err) {
			return CreateTransferResponse{}, fmt.Errorf("%w: transaction_id already exists", ErrInvalidRequest)
		}
		return CreateTransferResponse{}, err
	}

	event := TransferEvent{
		TransactionID: txID,
		FromAccountID: fromID,
		ToAccountID:   toID,
		Amount:        req.Amount,
	}
	if s.producer == nil {
		return CreateTransferResponse{}, errors.New("producer is not configured")
	}
	if err := s.producer.PublishTransfer(ctx, event); err != nil {
		return CreateTransferResponse{}, err
	}

	return CreateTransferResponse{
		TransactionID: txID,
		Status:        StatusProcessing,
		Message:       "Transfer accepted for async processing",
	}, nil
}

func (s *Service) GetTransfer(ctx context.Context, id uuid.UUID) (Transaction, error) {
	return s.transactions.GetByID(ctx, id)
}

func (s *Service) ProcessTransfer(ctx context.Context, event TransferEvent) (ProcessResult, error) {
	dbtx, err := s.db.Begin(ctx)
	if err != nil {
		return ProcessResult{}, err
	}
	defer func() {
		if err := dbtx.Rollback(ctx); err != nil {
			slog.Debug("rollback skipped or failed", "error", err)
		}
	}()

	tx, err := s.transactions.GetByIDForUpdate(ctx, dbtx, event.TransactionID)
	if err != nil {
		return ProcessResult{}, err
	}

	if tx.Status == StatusCompleted {
		if err := dbtx.Commit(ctx); err != nil {
			return ProcessResult{}, err
		}
		return ProcessResult{Status: tx.Status, Message: "duplicate message ignored"}, nil
	}
	if tx.Status == StatusFailed {
		if err := dbtx.Commit(ctx); err != nil {
			return ProcessResult{}, err
		}
		return ProcessResult{Status: tx.Status, Message: "already failed, message ignored"}, nil
	}
	if tx.Status != StatusProcessing {
		return ProcessResult{}, fmt.Errorf("unexpected transaction status: %s", tx.Status)
	}

	from, to, err := s.accounts.GetManyForUpdate(ctx, dbtx, tx.FromAccountID, tx.ToAccountID)
	if err != nil {
		return ProcessResult{}, err
	}

	if from.Balance < tx.Amount {
		if err := s.transactions.UpdateStatus(ctx, dbtx, tx.ID, StatusFailed); err != nil {
			return ProcessResult{}, err
		}
		if err := dbtx.Commit(ctx); err != nil {
			return ProcessResult{}, err
		}
		return ProcessResult{Status: StatusFailed, Message: ErrInsufficientBalance.Error()}, nil
	}

	if err := s.accounts.UpdateBalance(ctx, dbtx, from.ID, from.Balance-tx.Amount); err != nil {
		return ProcessResult{}, err
	}
	if err := s.accounts.UpdateBalance(ctx, dbtx, to.ID, to.Balance+tx.Amount); err != nil {
		return ProcessResult{}, err
	}
	if err := s.transactions.UpdateStatus(ctx, dbtx, tx.ID, StatusCompleted); err != nil {
		return ProcessResult{}, err
	}
	if err := dbtx.Commit(ctx); err != nil {
		return ProcessResult{}, err
	}

	return ProcessResult{Status: StatusCompleted, Message: "transfer completed"}, nil
}

func (s *Service) ensureAccountsExist(ctx context.Context, fromID, toID uuid.UUID) error {
	fromExists, err := s.accounts.Exists(ctx, fromID)
	if err != nil {
		return err
	}
	if !fromExists {
		return fmt.Errorf("%w: from_account_id does not exist", ErrInvalidRequest)
	}

	toExists, err := s.accounts.Exists(ctx, toID)
	if err != nil {
		return err
	}
	if !toExists {
		return fmt.Errorf("%w: to_account_id does not exist", ErrInvalidRequest)
	}

	return nil
}

func parseRequiredUUID(value string, field string) (uuid.UUID, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return uuid.Nil, fmt.Errorf("%w: %s is required", ErrInvalidRequest, field)
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: %s must be a valid uuid", ErrInvalidRequest, field)
	}
	return id, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
