package db

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func newTestRepo(t *testing.T) (*FilesRepo, *pgxpool.Pool, context.Context) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
	})
	return NewFilesRepo(pool), pool, ctx
}

func TestFileRepo_Create(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()

	pool, err := New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	repo := NewFilesRepo(pool)
	id, err := repo.Create(ctx, "test.txt", 123, []byte("hash-placeholder"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == "" {
		t.Fatalf("expected non-empty id")
	}

	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM files WHERE id = $1`, id)
	})
}

func TestFilesRepo_Claim(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()

	pool, err := New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	repo := NewFilesRepo(pool)

	if _, err := pool.Exec(ctx, `DELETE FROM files WHERE status = 'pending'`); err != nil {
		t.Fatal(err)
	}

	id, err := repo.Create(ctx, "claim-test.txt", 10, []byte("h"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM files WHERE id=$1`, id)
	})

	task, err := repo.Claim(ctx)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if task == nil {
		t.Fatal("we were expecting a task, but we got nil")
	}
	if task.ID != id {
		t.Errorf("captured the wrong one: got %s, want%s", task.ID, id)
	}

	again, err := repo.Claim(ctx)
	if err != nil {
		t.Fatalf("Claim 2: %v", err)
	}
	if again != nil {
		t.Fatalf("the task has already been captured, the second Claim should return nil, received %s", again.ID)
	}

}

func TestFilesRepo_MarkDone(t *testing.T) {
	repo, pool, ctx := newTestRepo(t)

	id, err := repo.Create(ctx, "done-test.txt", 100, []byte("h"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM files WHERE id=$1`, id) })

	if err := repo.MarkDone(ctx, id, 42); err != nil {
		t.Fatalf("MarkDone: %v", err)
	}

	var status string
	var sizeCompressed int64
	err = pool.QueryRow(ctx, `SELECT status, size_compressed FROM files WHERE id=$1`, id).Scan(&status, &sizeCompressed)
	if err != nil {
		t.Fatal(err)
	}
	if status != "done" {
		t.Errorf("status = %q, want done", status)
	}
	if sizeCompressed != 42 {
		t.Errorf("size_compressed = %d, want 42", sizeCompressed)
	}
}

func TestFilesRepo_MarkError(t *testing.T) {
	repo, pool, ctx := newTestRepo(t)

	id, err := repo.Create(ctx, "err-test.txt", 100, []byte("h"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM files WHERE id=$1`, id) })

	if err := repo.MarkError(ctx, id, "boom"); err != nil {
		t.Fatalf("MarkError: %v", err)
	}

	var status, reason string
	err = pool.QueryRow(ctx, `SELECT status, error FROM files WHERE id=$1`, id).Scan(&status, &reason)
	if err != nil {
		t.Fatal(err)
	}
	if status != "error" {
		t.Errorf("status = %q, want error", status)
	}
	if reason != "boom" {
		t.Errorf("error = %q, want boom", reason)
	}
}

func TestFilesRepo_ResetStuck(t *testing.T) {
	repo, pool, ctx := newTestRepo(t)

	if _, err := pool.Exec(ctx, `DELETE FROM files WHERE status IN ('pending', 'processing')`); err != nil {
		t.Fatal(err)
	}

	id, err := repo.Create(ctx, "stuck-test.txt", 100, []byte("h"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM files WHERE id=$1`, id) })

	task, err := repo.Claim(ctx)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if task == nil || task.ID != id {
		t.Fatal("Claim did not return our issue")
	}

	n, err := repo.ResetStuck(ctx)
	if err != nil {
		t.Fatalf("ResetStuck: %v", err)
	}
	if n < 1 {
		t.Fatalf("ResetStuck returned %d, expected >= 1", n)
	}

	task2, err := repo.Claim(ctx)
	if err != nil {
		t.Fatalf("Claim 2: %v", err)
	}
	if task2 == nil || task2.ID != id {
		t.Fatal("after ResetStuck, the task did not return to the queue")
	}
}
