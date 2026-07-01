package db

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
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

// RecalcularSaldos recalcula el saldo corrido de caja o bolsillo desde la fecha dada.
// Debe llamarse dentro de la misma transacción después de cualquier INSERT.
func RecalcularSaldos(ctx context.Context, tx pgx.Tx, tabla string, fecha string) error {
	_, err := tx.Exec(ctx, `
		WITH saldo_base AS (
			SELECT COALESCE(
				(SELECT saldo FROM `+tabla+` WHERE fecha < $1::date ORDER BY fecha DESC, id DESC LIMIT 1),
				0
			) AS base
		),
		recalculo AS (
			SELECT c.id,
				SUM(c.entrada - c.salida) OVER (ORDER BY c.fecha, c.id ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW)
				+ sb.base AS nuevo_saldo
			FROM `+tabla+` c, saldo_base sb
			WHERE c.fecha >= $1::date
		)
		UPDATE `+tabla+` SET saldo = recalculo.nuevo_saldo
		FROM recalculo WHERE `+tabla+`.id = recalculo.id
	`, fecha)
	return err
}
