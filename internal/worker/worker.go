package worker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/restorte/lzhuff-store/internal/codec"
	"github.com/restorte/lzhuff-store/internal/db"
	"github.com/restorte/lzhuff-store/internal/storage"
)

const pollInterval = time.Second

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

	if err := w.verify(task); err != nil {
		w.fail(ctx, task.ID, err)
		return true, nil
	}

	if err := w.repo.MarkDone(ctx, task.ID, int64(len(comp))); err != nil {
		return true, fmt.Errorf("worker: mark done: %w", err)
	}

	if err := w.originals.Delete(task.ID); err != nil {
		log.Printf("worker: delete original %s: %v", task.ID, err)
	}
	return true, nil
}

func (w *Worker) verify(task *db.Task) error {
	stored, err := w.containers.Read(task.ID)
	if err != nil {
		return fmt.Errorf("verify: read container: %w", err)
	}
	back, err := codec.Decompress(stored)
	if err != nil {
		return fmt.Errorf("verify: decompress: %w", err)
	}
	sum := sha256.Sum256(back)
	if !bytes.Equal(sum[:], task.SHA256) {
		return errors.New("verify: checksum mismatch after round-trip")
	}
	return nil
}

func (w *Worker) Run(ctx context.Context, n int) error {
	reset, err := w.repo.ResetStuck(ctx)
	if err != nil {
		return fmt.Errorf("worker: reset stuck: %w", err)
	}
	if reset > 0 {
		log.Printf("worker: recovered %d stuck task(s)", reset)
	}

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			w.loop(ctx, id)
		}(i)
	}
	wg.Wait()
	return nil
}

func (w *Worker) loop(ctx context.Context, id int) {
	for {
		if ctx.Err() != nil {
			return
		}

		worked, err := w.processOne(ctx)
		if err != nil {
			log.Printf("worker %d: %v", id, err)
		}
		if worked {
			continue
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(pollInterval):
		}
	}
}
