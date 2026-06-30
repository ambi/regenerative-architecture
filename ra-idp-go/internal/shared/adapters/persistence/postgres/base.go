// Package postgres: 永続化アダプタの PostgreSQL 実装。
// 接続を base.go に置き、リポジトリ実装は
// 境界づけられたコンテキスト単位でファイル分割している (tenants.go / clients.go ...)。
package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	config.MaxConns = 20
	config.MaxConnIdleTime = 30 * time.Second
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET statement_timeout = '5s'; SET idle_in_transaction_session_timeout = '30s'")
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// rowScanner は pgx.Row / pgx.Rows の共通スキャンインターフェース。
type rowScanner interface{ Scan(...any) error }
