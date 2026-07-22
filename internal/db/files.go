package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FilesRepo struct {
	pool *pgxpool.Pool
}

type Task struct {
	ID           string
	Name         string
	SizeOriginal int64
	SHA256       []byte
}

func NewFilesRepo(pool *pgxpool.Pool) *FilesRepo {
	return &FilesRepo{pool: pool}
}

func (r *FilesRepo) Create(ctx context.Context, name string, sizeOriginal int64, sha256 []byte) (string, error) {
	const q = `INSERT INTO files (name, size_original, sha256)
			   VALUES ($1, $2, $3)
			   RETURNING id`

	var id string
	err := r.pool.QueryRow(ctx, q, name, sizeOriginal, sha256).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("file create: %w", err)
	}

	return id, nil
}

func (r *FilesRepo) Claim(ctx context.Context) (*Task, error) {
	const q = `UPDATE files
			   SET status = 'processing', updated_at = now()
			   WHERE id = (
			    SELECT id FROM files
				WHERE status = 'pending'
				ORDER BY created_at
				FOR UPDATE SKIP LOCKED
				LIMIT 1
			   )
			   RETURNING id, name, size_original, sha256`

	var t Task
	err := r.pool.QueryRow(ctx, q).Scan(&t.ID, &t.Name, &t.SizeOriginal, &t.SHA256)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("files: claim: %w", err)
	}

	return &t, nil
}
