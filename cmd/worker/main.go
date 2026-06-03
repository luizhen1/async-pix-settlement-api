package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"async-pix-settlement-api/internal/account"
	"async-pix-settlement-api/internal/config"
	"async-pix-settlement-api/internal/database"
	"async-pix-settlement-api/internal/kafka"
	"async-pix-settlement-api/internal/transaction"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	accountRepo := account.NewPostgresRepository(db)
	transactionRepo := transaction.NewPostgresRepository(db)
	service := transaction.NewService(transactionRepo, accountRepo, nil, db)

	consumer := kafka.NewConsumer(cfg.KafkaBrokers, cfg.KafkaTopic, "pix-settlement-worker")
	defer func() {
		if err := consumer.Close(); err != nil {
			logger.Error("failed to close kafka consumer", "error", err)
		}
	}()

	logger.Info("worker started", "topic", cfg.KafkaTopic)
	if err := consumer.Consume(ctx, func(ctx context.Context, event transaction.TransferEvent) error {
		result, err := service.ProcessTransfer(ctx, event)
		if err != nil {
			logger.Error("transfer processing failed",
				"transaction_id", event.TransactionID,
				"from_account_id", event.FromAccountID,
				"to_account_id", event.ToAccountID,
				"error", err,
			)
			return err
		}

		logger.Info("transfer processing finished",
			"transaction_id", event.TransactionID,
			"status", result.Status,
			"message", result.Message,
		)
		return nil
	}); err != nil && ctx.Err() == nil {
		logger.Error("consumer stopped with error", "error", err)
		os.Exit(1)
	}
}
