// Package testutil holds helpers shared by the integration tests.
package testutil

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

const testLockID = 8675309

func LockDB(ctx context.Context, pool *pgxpool.Pool) (release func(), err error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, testLockID); err != nil {
		conn.Release()
		return nil, err
	}
	return func() {
		conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, testLockID)
		conn.Release()
	}, nil
}
