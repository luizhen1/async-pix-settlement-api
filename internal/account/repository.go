package account

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrAccountNotFound = errors.New("account not found")

type Repository interface {
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
	List(ctx context.Context) ([]Account, error)
	GetManyForUpdate(ctx context.Context, tx pgx.Tx, fromID, toID uuid.UUID) (Account, Account, error)
	UpdateBalance(ctx context.Context, tx pgx.Tx, id uuid.UUID, balance float64) error
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM accounts WHERE id = $1)`, id).Scan(&exists)
	return exists, err
}

func (r *PostgresRepository) List(ctx context.Context) ([]Account, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, owner_name, balance::float8, created_at, updated_at
		FROM accounts
		ORDER BY owner_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	accounts := make([]Account, 0)
	for rows.Next() {
		var account Account
		if err := rows.Scan(&account.ID, &account.OwnerName, &account.Balance, &account.CreatedAt, &account.UpdatedAt); err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}

	return accounts, rows.Err()
}

func (r *PostgresRepository) GetManyForUpdate(ctx context.Context, tx pgx.Tx, fromID, toID uuid.UUID) (Account, Account, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, owner_name, balance::float8, created_at, updated_at
		FROM accounts
		WHERE id = $1 OR id = $2
		ORDER BY id
		FOR UPDATE
	`, fromID, toID)
	if err != nil {
		return Account{}, Account{}, err
	}
	defer rows.Close()

	accounts := make(map[uuid.UUID]Account, 2)
	for rows.Next() {
		var account Account
		if err := rows.Scan(&account.ID, &account.OwnerName, &account.Balance, &account.CreatedAt, &account.UpdatedAt); err != nil {
			return Account{}, Account{}, err
		}
		accounts[account.ID] = account
	}
	if err := rows.Err(); err != nil {
		return Account{}, Account{}, err
	}

	from, ok := accounts[fromID]
	if !ok {
		return Account{}, Account{}, ErrAccountNotFound
	}
	to, ok := accounts[toID]
	if !ok {
		return Account{}, Account{}, ErrAccountNotFound
	}

	return from, to, nil
}

func (r *PostgresRepository) UpdateBalance(ctx context.Context, tx pgx.Tx, id uuid.UUID, balance float64) error {
	commandTag, err := tx.Exec(ctx, `
		UPDATE accounts
		SET balance = $2, updated_at = now()
		WHERE id = $1
	`, id, balance)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrAccountNotFound
	}
	return nil
}
