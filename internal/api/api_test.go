package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/restorte/lzhuff-store/internal/codec"
	"github.com/restorte/lzhuff-store/internal/db"
	"github.com/restorte/lzhuff-store/internal/storage"
	"github.com/restorte/lzhuff-store/internal/testutil"
)

func newTestAPI(t *testing.T) (*API, *db.FilesRepo, *pgxpool.Pool, *storage.Storage, *storage.Storage, context.Context) {
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

	release, err := testutil.LockDB(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)

	repo := db.NewFilesRepo(pool)
	originals, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	containers, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return New(repo, originals, containers), repo, pool, originals, containers, ctx
}

func TestAPI_UploadThenDownload(t *testing.T) {
	api, repo, pool, originals, containers, ctx := newTestAPI(t)

	raw := []byte("hello hello hello world world world api round-trip test test")

	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/files?name=t.txt", bytes.NewReader(raw)))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("upload status = %d, want 202", rec.Code)
	}
	var up struct{ ID string }
	if err := json.NewDecoder(rec.Body).Decode(&up); err != nil {
		t.Fatal(err)
	}
	if up.ID == "" {
		t.Fatal("empty id in upload response")
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM files WHERE id=$1`, up.ID) })

	if got, err := originals.Read(up.ID); err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("original not stored: err=%v", err)
	}

	rec = httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/files/"+up.ID, nil))
	if rec.Code != http.StatusAccepted {
		t.Errorf("get-while-pending status = %d, want 202", rec.Code)
	}

	comp, err := codec.Compress(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := containers.Write(up.ID, comp); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkDone(ctx, up.ID, int64(len(comp))); err != nil {
		t.Fatal(err)
	}

	rec = httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/files/"+up.ID, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get-done status = %d, want 200", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if !bytes.Equal(body, raw) {
		t.Errorf("download = %q, want %q", body, raw)
	}
}

func TestAPI_DownloadCarriesFilename(t *testing.T) {
	api, repo, pool, _, containers, ctx := newTestAPI(t)

	raw := []byte("id,name\n1,alice\n2,bob\n")
	sum := sha256.Sum256(raw)

	id, err := repo.Create(ctx, "report.csv", int64(len(raw)), sum[:])
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM files WHERE id=$1`, id) })

	comp, err := codec.Compress(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := containers.Write(id, comp); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkDone(ctx, id, int64(len(comp))); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/files/"+id, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	cd := rec.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "report.csv") {
		t.Errorf("Content-Disposition = %q, want it to name report.csv", cd)
	}
	if ct := rec.Header().Get("Content-Type"); ct == "" {
		t.Error("Content-Type is empty")
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing nosniff header on user-supplied content")
	}
}

func TestAPI_DownloadSanitizesFilename(t *testing.T) {
	rec := httptest.NewRecorder()
	setDownloadHeaders(rec, "../../etc/\"passwd\"\nX-Evil: yes", "fallback-id")

	cd := rec.Header().Get("Content-Disposition")
	if strings.Contains(cd, "..") || strings.Contains(cd, "/") {
		t.Errorf("path not stripped from filename: %q", cd)
	}
	if strings.Contains(cd, "\n") {
		t.Errorf("newline survived into header: %q", cd)
	}
	if rec.Header().Get("X-Evil") != "" {
		t.Error("attacker managed to inject an extra header")
	}
}

func TestAPI_UploadRejectsOversizeBody(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI(t)
	api.maxUpload = 64

	body := bytes.Repeat([]byte("x"), 200)
	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/files?name=big.bin", bytes.NewReader(body)))

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", rec.Code)
	}
}

func TestAPI_GetInvalidID(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI(t)

	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/files/not-a-uuid", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("invalid id status = %d, want 400", rec.Code)
	}
}

func TestAPI_GetNotFound(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI(t)

	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/files/00000000-0000-0000-0000-000000000000", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("absent id status = %d, want 404", rec.Code)
	}
}
