package transaction

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrTransactionNotFound = errors.New("transaction not found")

type Repository interface {
	Create(ctx context.Context, tx Transaction) error
	GetByID(ctx context.Context, id uuid.UUID) (Transaction, error)
	GetByIDForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Transaction, error)
	UpdateStatus(ctx context.Context, tx pgx.Tx, id uuid.UUID, status Status) error
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(ctx context.Context, tx Transaction) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO transactions (id, from_account_id, to_account_id, amount_cents, status)
		VALUES ($1, $2, $3, $4, $5)
	`, tx.ID, tx.FromAccountID, tx.ToAccountID, tx.AmountCents, tx.Status)
	return err
}

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (Transaction, error) {
	return scanTransaction(r.db.QueryRow(ctx, `
		SELECT id, from_account_id, to_account_id, amount_cents, status, created_at, updated_at
		FROM transactions
		WHERE id = $1
	`, id))
}

func (r *PostgresRepository) GetByIDForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Transaction, error) {
	return scanTransaction(tx.QueryRow(ctx, `
		SELECT id, from_account_id, to_account_id, amount_cents, status, created_at, updated_at
		FROM transactions
		WHERE id = $1
		FOR UPDATE
	`, id))
}

func (r *PostgresRepository) UpdateStatus(ctx context.Context, tx pgx.Tx, id uuid.UUID, status Status) error {
	commandTag, err := tx.Exec(ctx, `
		UPDATE transactions
		SET status = $2, updated_at = now()
		WHERE id = $1
	`, id, status)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrTransactionNotFound
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanTransaction(row scanner) (Transaction, error) {
	var tx Transaction
	if err := row.Scan(&tx.ID, &tx.FromAccountID, &tx.ToAccountID, &tx.AmountCents, &tx.Status, &tx.CreatedAt, &tx.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Transaction{}, ErrTransactionNotFound
		}
		return Transaction{}, err
	}
	return tx, nil
}
