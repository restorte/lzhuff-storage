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
		t.Fatal("ожидали задачу, получили nil")
	}
	if task.ID != id {
		t.Errorf("захватили не ту: got %s, want %s", task.ID, id)
	}

	again, err := repo.Claim(ctx)
	if err != nil {
		t.Fatalf("Claim 2: %v", err)
	}
	if again != nil {
		t.Fatalf("задача уже захвачена, второй Claim должен вернуть nil, получили %s", again.ID)
	}

}
