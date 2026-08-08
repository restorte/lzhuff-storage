package api

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/restorte/lzhuff-storage/internal/codec"
	"github.com/restorte/lzhuff-storage/internal/db"
	"github.com/restorte/lzhuff-storage/internal/storage"
)

const (
	defaultListLimit = 50
	maxListLimit     = 200
	defaultMaxUpload = 32 << 20
)

type API struct {
	repo       *db.FilesRepo
	originals  *storage.Storage
	containers *storage.Storage
	maxUpload  int64
}

func New(repo *db.FilesRepo, originals, containers *storage.Storage, maxUpload int64) *API {
	if maxUpload <= 0 {
		maxUpload = defaultMaxUpload
	}
	return &API{
		repo:       repo,
		originals:  originals,
		containers: containers,
		maxUpload:  maxUpload,
	}
}

func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /files", a.handleUpload)
	mux.HandleFunc("GET /files", a.handleList)
	mux.HandleFunc("GET /files/{id}", a.handleGet)
	mux.HandleFunc("DELETE /files/{id}", a.handleDelete)
	return mux
}

func (a *API) handleUpload(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")

	r.Body = http.MaxBytesReader(w, r.Body, a.maxUpload)

	f, err := a.originals.CreatePending()
	if err != nil {
		http.Error(w, "store original", http.StatusInternalServerError)
		return
	}
	defer f.Abort()

	sum := sha256.New()
	size, err := io.Copy(io.MultiWriter(f, sum), r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	id, err := a.repo.Create(r.Context(), name, size, sum.Sum(nil))
	if err != nil {
		http.Error(w, "create record", http.StatusInternalServerError)
		return
	}

	if err := f.CommitAs(id); err != nil {
		a.repo.Delete(r.Context(), id)
		http.Error(w, "store original", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

func (a *API) handleList(w http.ResponseWriter, r *http.Request) {
	limit := defaultListLimit
	if s := r.URL.Query().Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n <= 0 || n > maxListLimit {
			http.Error(w, "limit must be a number between 1 and "+strconv.Itoa(maxListLimit), http.StatusBadRequest)
			return
		}
		limit = n
	}

	files, err := a.repo.List(r.Context(), limit)
	if err != nil {
		http.Error(w, "list", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

func (a *API) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	existed, err := a.repo.Delete(r.Context(), id)
	if errors.Is(err, db.ErrInvalidID) {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if errors.Is(err, db.ErrBusy) {
		http.Error(w, "file is being processed, try again shortly", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, "delete", http.StatusInternalServerError)
		return
	}
	if !existed {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if err := a.originals.Delete(id); err != nil {
		log.Printf("api: delete original %s: %v", id, err)
	}
	if err := a.containers.Delete(id); err != nil {
		log.Printf("api: delete container %s: %v", id, err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func setDownloadHeaders(w http.ResponseWriter, name, fallback string) {
	name = filepath.Base(name)
	if name == "." || name == string(filepath.Separator) || name == "" {
		name = fallback
	}

	ct := mime.TypeByExtension(filepath.Ext(name))
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if cd := mime.FormatMediaType("attachment", map[string]string{"filename": name}); cd != "" {
		w.Header().Set("Content-Disposition", cd)
	} else {
		w.Header().Set("Content-Disposition", "attachment")
	}
}

func (a *API) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	f, err := a.repo.Get(r.Context(), id)
	if errors.Is(err, db.ErrInvalidID) {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "lookup", http.StatusInternalServerError)
		return
	}
	if f == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	switch f.Status {
	case "done":
		src, err := a.containers.Open(id)
		if err != nil {
			http.Error(w, "read container", http.StatusInternalServerError)
			return
		}
		defer src.Close()

		setDownloadHeaders(w, f.Name, id)
		if err := codec.DecompressStream(src, w); err != nil {
			log.Printf("api: decompress %s: %v", id, err)
			return
		}
	case "error":
		http.Error(w, "processing failed: "+f.Error, http.StatusInternalServerError)
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": f.Status})
	}
}
