package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type FilesRepo struct {
	pool *pgxpool.Pool
}

func NewFilesRepo(pool *pgxpool.Pool) *FilesRepo {
	return &FilesRepo{pool: pool}
}

func (r *FilesRepo) Create(ctx context.Context, name string, sizeOriginal int64, sha256 []byte) (id string, err error) {
	const q = `INSERT INTO files (name, size_original, sha256)
			   VALUES ($1, $2, $3)
			   RETURNING id`

	err = r.pool.QueryRow(ctx, q, name, sizeOriginal, sha256).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("file create: %w", err)
	}

	return id, nil
}
