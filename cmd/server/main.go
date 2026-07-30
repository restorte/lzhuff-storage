package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/restorte/lzhuff-store/internal/api"
	"github.com/restorte/lzhuff-store/internal/db"
	"github.com/restorte/lzhuff-store/internal/storage"
	"github.com/restorte/lzhuff-store/internal/worker"
)

func envInt(name string, def int) int {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		log.Fatalf("%s must be a positive integer, got %q", name, v)
	}
	return n
}

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}
	root := os.Getenv("STORAGE_ROOT")
	if root == "" {
		root = "storage"
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	workers := envInt("WORKERS", runtime.NumCPU())
	maxUploadMB := envInt("MAX_UPLOAD_MB", 32)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.New(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	repo := db.NewFilesRepo(pool)

	originals, err := storage.New(filepath.Join(root, "originals"))
	if err != nil {
		log.Fatal(err)
	}
	containers, err := storage.New(filepath.Join(root, "containers"))
	if err != nil {
		log.Fatal(err)
	}

	w := worker.New(repo, originals, containers)

	if err := w.Startup(ctx); err != nil {
		log.Fatal(err)
	}

	workerDone := make(chan struct{})
	go func() {
		if err := w.Run(ctx, workers); err != nil {
			log.Printf("worker: %v", err)
		}
		close(workerDone)
	}()

	srv := &http.Server{
		Addr:    addr,
		Handler: api.New(repo, originals, containers, int64(maxUploadMB)<<20).Routes(),
	}
	go func() {
		log.Printf("listening on %s (workers=%d, max upload=%d MiB)", addr, workers, maxUploadMB)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown: %v", err)
	}

	<-workerDone
	log.Println("stopped")
}
