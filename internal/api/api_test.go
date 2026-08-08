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

	"github.com/restorte/lzhuff-storage/internal/codec"
	"github.com/restorte/lzhuff-storage/internal/db"
	"github.com/restorte/lzhuff-storage/internal/storage"
	"github.com/restorte/lzhuff-storage/internal/testutil"
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
	return New(repo, originals, containers, defaultMaxUpload), repo, pool, originals, containers, ctx
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

func TestAPI_NewFallsBackToDefaultLimit(t *testing.T) {
	for _, given := range []int64{0, -1} {
		api := New(nil, nil, nil, given)
		if api.maxUpload != defaultMaxUpload {
			t.Errorf("New(..., %d) set maxUpload=%d, want the default %d", given, api.maxUpload, defaultMaxUpload)
		}
	}
}

func TestAPI_Delete(t *testing.T) {
	api, repo, pool, originals, containers, ctx := newTestAPI(t)

	id, err := repo.Create(ctx, "doomed.txt", 3, []byte("h"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM files WHERE id=$1`, id) })
	if err := originals.Write(id, []byte("abc")); err != nil {
		t.Fatal(err)
	}
	if err := containers.Write(id, []byte("abc")); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/files/"+id, nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("204 must carry no body, got %q", rec.Body.String())
	}

	if f, err := repo.Get(ctx, id); err != nil || f != nil {
		t.Errorf("row survived the delete: %v, %v", f, err)
	}
	if _, err := originals.Read(id); err == nil {
		t.Error("original survived the delete")
	}
	if _, err := containers.Read(id); err == nil {
		t.Error("container survived the delete")
	}

	rec = httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/files/"+id, nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("repeat delete status = %d, want 404", rec.Code)
	}
}

func TestAPI_DeleteRefusesWhileProcessing(t *testing.T) {
	api, repo, pool, _, _, ctx := newTestAPI(t)

	id, err := repo.Create(ctx, "busy.txt", 3, []byte("h"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM files WHERE id=$1`, id) })

	if _, err := pool.Exec(ctx, `UPDATE files SET status='processing' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/files/"+id, nil))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}

	if f, err := repo.Get(ctx, id); err != nil || f == nil {
		t.Errorf("row was removed despite being in processing: %v, %v", f, err)
	}
}

func TestAPI_DeleteInvalidID(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI(t)

	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/files/not-a-uuid", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestAPI_List(t *testing.T) {
	api, repo, pool, _, _, ctx := newTestAPI(t)

	id, err := repo.Create(ctx, "listed.txt", 42, []byte("h"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM files WHERE id=$1`, id) })

	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/files", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var got []db.FileInfo
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}

	var found *db.FileInfo
	for i := range got {
		if got[i].ID == id {
			found = &got[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("the file just created is missing from the listing of %d", len(got))
	}
	if found.Name != "listed.txt" || found.Status != "pending" || found.SizeOriginal != 42 {
		t.Errorf("unexpected entry: %+v", *found)
	}
	if found.SizeCompressed != nil {
		t.Errorf("size_compressed = %v, want null for a pending file", *found.SizeCompressed)
	}
}

func TestAPI_ListRejectsBadLimit(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI(t)

	for _, limit := range []string{"0", "-1", "abc", "9999"} {
		rec := httptest.NewRecorder()
		api.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/files?limit="+limit, nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("limit=%s: status = %d, want 400", limit, rec.Code)
		}
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
