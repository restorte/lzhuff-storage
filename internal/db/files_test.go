package db

import (
	"context"
	"os"
	"testing"
)

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
