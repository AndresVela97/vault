package db

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

var Pool *pgxpool.Pool

func Connect() error {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return fmt.Errorf("DATABASE_URL no configurado")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		return fmt.Errorf("error conectando a PostgreSQL: %w", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		return fmt.Errorf("error en ping a PostgreSQL: %w", err)
	}
	Pool = pool
	return nil
}
