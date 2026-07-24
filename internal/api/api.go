package api

import (
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"

	"github.com/restorte/lzhuff-store/internal/db"
	"github.com/restorte/lzhuff-store/internal/storage"
)

type API struct {
	repo       *db.FilesRepo
	originals  *storage.Storage
	containers *storage.Storage
}

func New(repo *db.FilesRepo, originals, containers *storage.Storage) *API {
	return &API{repo: repo, originals: originals, containers: containers}
}

func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /files", a.handleUpload)
	return mux
}

func (a *API) handleUpload(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")

	body, err := io.ReadAll(r.Body)
	if err != nil {
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
