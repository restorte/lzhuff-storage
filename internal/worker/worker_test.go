package worker

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/restorte/lzhuff-store/internal/codec"
	"github.com/restorte/lzhuff-store/internal/db"
	"github.com/restorte/lzhuff-store/internal/storage"
)

func newTestWorker(t *testing.T) (*Worker, *db.FilesRepo, *pgxpool.Pool, *storage.Storage, *storage.Storage, context.Context) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()

	pool, err := db.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close() })

	repo := db.NewFilesRepo(pool)
	originals, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	containers, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := pool.Exec(ctx, `DELETE FROM files WHERE status IN ('pending', 'processing')`); err != nil {
		t.Fatal(err)
	}

	return New(repo, originals, containers), repo, pool, originals, containers, ctx
}

func TestWorker_ProcessOne_Success(t *testing.T) {
	w, repo, pool, originals, containers, ctx := newTestWorker(t)

	raw := []byte("hello hello hello world world world")

	id, err := repo.Create(ctx, "wtest.txt", int64(len(raw)), []byte("sha-placeholder"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM files WHERE id=$1`, id) })

	if err := originals.Write(id, raw); err != nil {
		t.Fatal(err)
	}

	worked, err := w.processOne(ctx)
	if err != nil {
		t.Fatalf("processOne: %v", err)
	}
	if !worked {
		t.Fatal("expected processOne to find the task")
	}

	var status string
	var sizeCompressed int64
	if err := pool.QueryRow(ctx, `SELECT status, size_compressed FROM files WHERE id=$1`, id).Scan(&status, &sizeCompressed); err != nil {
		t.Fatal(err)
	}
	if status != "done" {
		t.Errorf("status = %q, want done", status)
	}

	comp, err := containers.Read(id)
	if err != nil {
		t.Fatalf("container not found: %v", err)
	}
	if int64(len(comp)) != sizeCompressed {
		t.Errorf("size_compressed = %d, but container is %d bytes", sizeCompressed, len(comp))
	}
	back, err := codec.Decompress(comp)
	if err != nil {
		t.Fatalf("Decompress: %v", err)
	}
	if !bytes.Equal(back, raw) {
		t.Errorf("round-trip through worker: got %q, want %q", back, raw)
	}
}

func TestWorker_ProcessOne_MissingOriginalMarksError(t *testing.T) {
	w, repo, pool, _, _, ctx := newTestWorker(t)

	id, err := repo.Create(ctx, "no-original.txt", 100, []byte("sha-placeholder"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM files WHERE id=$1`, id) })

	worked, err := w.processOne(ctx)
	if err != nil {
		t.Fatalf("processOne returned a system error: %v", err)
	}
	if !worked {
		t.Fatal("expected processOne to find the task")
	}

	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM files WHERE id=$1`, id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "error" {
		t.Errorf("status = %q, want error (task must leave processing, not hang)", status)
	}
}
