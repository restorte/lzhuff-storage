package api

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"path/filepath"

	"github.com/restorte/lzhuff-store/internal/codec"
	"github.com/restorte/lzhuff-store/internal/db"
	"github.com/restorte/lzhuff-store/internal/storage"
)

const defaultMaxUpload = 32 << 20

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
	mux.HandleFunc("GET /files/{id}", a.handleGet)
	return mux
}

func (a *API) handleUpload(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")

	r.Body = http.MaxBytesReader(w, r.Body, a.maxUpload)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	sum := sha256.Sum256(body)

	id, err := a.repo.Create(r.Context(), name, int64(len(body)), sum[:])
	if err != nil {
		http.Error(w, "create record", http.StatusInternalServerError)
		return
	}

	if err := a.originals.Write(id, body); err != nil {
		http.Error(w, "store original", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"id": id})
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
		comp, err := a.containers.Read(id)
		if err != nil {
			http.Error(w, "read container", http.StatusInternalServerError)
			return
		}
		raw, err := codec.Decompress(comp)
		if err != nil {
			http.Error(w, "decompress", http.StatusInternalServerError)
			return
		}
		setDownloadHeaders(w, f.Name, id)
		w.Write(raw)
	case "error":
		http.Error(w, "processing failed: "+f.Error, http.StatusInternalServerError)
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": f.Status})
	}
}
