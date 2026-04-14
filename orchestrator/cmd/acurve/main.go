package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/Konk32/acurve/orchestrator/internal/api"
	"github.com/Konk32/acurve/orchestrator/internal/db"
	"github.com/Konk32/acurve/orchestrator/internal/discord"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	dbURL := mustEnv("DB_URL")
	port := envOr("PORT", "8080")
	migrationsDir := envOr("MIGRATIONS_DIR", "../migrations")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Run migrations via goose (idempotent).
	if err := runMigrations(dbURL, migrationsDir); err != nil {
		return fmt.Errorf("migrations: %w", err)
	}

	// Application pool (pgx native).
	pool, err := db.NewPool(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("db pool: %w", err)
	}
	defer pool.Close()

	store := db.NewStore(pool)

	var discordClient *discord.Client
	if webhookURL := os.Getenv("DISCORD_WEBHOOK_URL"); webhookURL != "" {
		discordClient = discord.NewClient(webhookURL)
	} else {
		slog.Warn("DISCORD_WEBHOOK_URL not set — digest send will be a no-op")
	}

	handler := api.NewServer(store, discordClient)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.Info("listening", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("listen: %w", err)
	}
	return nil
}

func runMigrations(dbURL, dir string) error {
	sqlDB, err := sql.Open("pgx", dbURL)
	if err != nil {
		return fmt.Errorf("open sql db: %w", err)
	}
	defer sqlDB.Close()

	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	if err := goose.Up(sqlDB, dir); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	slog.Info("migrations applied")
	return nil
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("missing required env var", "key", key)
		os.Exit(1)
	}
	return v
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
