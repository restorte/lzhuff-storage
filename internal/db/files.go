package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidID = errors.New("invalid id")
	ErrBusy      = errors.New("file is being processed")
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

type File struct {
	ID     string
	Name   string
	Status string
	Error  string
}

type FileInfo struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Status         string    `json:"status"`
	SizeOriginal   int64     `json:"size_original"`
	SizeCompressed *int64    `json:"size_compressed"`
	CreatedAt      time.Time `json:"created_at"`
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

func (r *FilesRepo) Get(ctx context.Context, id string) (*File, error) {
	const q = `SELECT id, name, status, COALESCE(error, '')
			   FROM files WHERE id = $1`

	var f File
	err := r.pool.QueryRow(ctx, q, id).Scan(&f.ID, &f.Name, &f.Status, &f.Error)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "22P02" {
		return nil, ErrInvalidID
	}
	if err != nil {
		return nil, fmt.Errorf("files: get: %w", err)
	}
	return &f, nil
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

func (r *FilesRepo) List(ctx context.Context, limit int) ([]FileInfo, error) {
	const q = `SELECT id, name, status, size_original, size_compressed, created_at
			   FROM files
			   ORDER BY created_at DESC
			   LIMIT $1`

	rows, err := r.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("files: list: %w", err)
	}
	defer rows.Close()

	files := make([]FileInfo, 0, limit)
	for rows.Next() {
		var f FileInfo
		if err := rows.Scan(&f.ID, &f.Name, &f.Status, &f.SizeOriginal, &f.SizeCompressed, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("files: list: scan: %w", err)
		}
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("files: list: %w", err)
	}
	return files, nil
}

func (r *FilesRepo) Delete(ctx context.Context, id string) (bool, error) {
	const q = `
		WITH target AS (
			SELECT id FROM files WHERE id = $1
		), removed AS (
			DELETE FROM files WHERE id = $1 AND status <> 'processing'
			RETURNING id
		)
		SELECT (SELECT count(*) FROM target), (SELECT count(*) FROM removed)`

	var existed, removed int
	err := r.pool.QueryRow(ctx, q, id).Scan(&existed, &removed)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "22P02" {
		return false, ErrInvalidID
	}
	if err != nil {
		return false, fmt.Errorf("files: delete: %w", err)
	}

	switch {
	case existed == 0:
		return false, nil
	case removed == 0:
		return false, ErrBusy
	default:
		return true, nil
	}
}

func (r *FilesRepo) AllIDs(ctx context.Context) (map[string]struct{}, error) {
	const q = `SELECT id FROM files`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("files: all ids: %w", err)
	}
	defer rows.Close()

	ids := make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("files: all ids: scan: %w", err)
		}
		ids[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("files: all ids: %w", err)
	}
	return ids, nil
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
