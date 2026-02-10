package investlog

import (
	"context"
	"database/sql"
)

// WithTx executes a function within a database transaction with proper error handling.
// It automatically handles transaction rollback on error and commit on success.
// Panics are caught and result in a rollback followed by re-panic.
func (c *Core) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return WrapError(ErrCodeDatabase, "failed to begin transaction", err)
	}

	defer func() {
		if p := recover(); p != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				c.logger.Error("transaction rollback failed on panic", "error", rbErr, "panic_value", p)
			}
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			c.logger.Error("transaction rollback failed", "error", rbErr, "original_error", err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return WrapError(ErrCodeDatabase, "failed to commit transaction", err)
	}

	return nil
}

// BeginTx starts a new database transaction with the given context.
// This is a lower-level method when you need more control over transaction lifecycle.
func (c *Core) BeginTx(ctx context.Context) (*sql.Tx, error) {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, WrapError(ErrCodeDatabase, "failed to begin transaction", err)
	}
	return tx, nil
}

// ExecContext executes a query with context without returning any rows.
func (c *Core) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	result, err := c.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, WrapError(ErrCodeDatabase, "query execution failed", err)
	}
	return result, nil
}

// QueryContext executes a query with context that returns rows.
func (c *Core) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	rows, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, WrapError(ErrCodeDatabase, "query execution failed", err)
	}
	return rows, nil
}

// QueryRowContext executes a query with context that returns at most one row.
func (c *Core) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return c.db.QueryRowContext(ctx, query, args...)
}
