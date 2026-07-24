package worker

import (
	"context"
	"fmt"
	"log"

	"github.com/restorte/lzhuff-store/internal/codec"
	"github.com/restorte/lzhuff-store/internal/db"
	"github.com/restorte/lzhuff-store/internal/storage"
)

type Worker struct {
	repo       *db.FilesRepo
	originals  *storage.Storage
	containers *storage.Storage
}

func New(repo *db.FilesRepo, originals, containers *storage.Storage) *Worker {
	return &Worker{repo: repo, originals: originals, containers: containers}
}

func (w *Worker) fail(ctx context.Context, id string, cause error) {
	if err := w.repo.MarkError(ctx, id, cause.Error()); err != nil {
		log.Printf("worker: mark error for %s failed: %v", id, err)
	}
}

func (w *Worker) processOne(ctx context.Context) (bool, error) {
	task, err := w.repo.Claim(ctx)
	if err != nil {
		return false, fmt.Errorf("worker: claim: %w", err)
	}
	if task == nil {
		return false, nil
	}

	raw, err := w.originals.Read(task.ID)
	if err != nil {
		w.fail(ctx, task.ID, err)
		return true, nil
	}

	comp, err := codec.Compress(raw)
	if err != nil {
		w.fail(ctx, task.ID, err)
		return true, nil
	}

	if err := w.containers.Write(task.ID, comp); err != nil {
		w.fail(ctx, task.ID, err)
		return true, nil
	}

	if err := w.repo.MarkDone(ctx, task.ID, int64(len(comp))); err != nil {
		return true, fmt.Errorf("worker: mark done: %w", err)
	}
	return true, nil
}
