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

func (r *FilesRepo) MarkDone(ctx context.Context, id string, sizeCompressed int64) error {
	const q = `UPDATE files
			   SET status = 'done', size_compressed = $2, updated_at = now()
			   WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id, sizeCompressed)
	if err != nil {
		return fmt.Errorf("files: mark done: %w", err)
	}
	return nil
}

func (r *FilesRepo) MarkError(ctx context.Context, id string, reason string) error {
	const q = `UPDATE files
			   SET status = 'error', error = $2, updated_at = now()
			   WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id, reason)
	if err != nil {
		return fmt.Errorf("files: mark error: %w", err)
	}
	return nil
}

func (r *FilesRepo) ResetStuck(ctx context.Context) (int64, error) {
	const q = `UPDATE files
			   SET status = 'pending', updated_at = now()
			   WHERE status = 'processing'`
	tag, err := r.pool.Exec(ctx, q)
	if err != nil {
		return 0, fmt.Errorf("files: reset stuck: %w", err)
	}
	return tag.RowsAffected(), nil
}
