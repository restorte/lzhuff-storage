package storage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const ext = ".lzh"

type Storage struct {
	root string
}

func New(root string) (*Storage, error) {
	err := os.MkdirAll(root, 0o755)
	if err != nil {
		return nil, fmt.Errorf("storage: new: %w", err)
	}
	return &Storage{root: root}, nil
}

func (s *Storage) path(id string) string {
	return filepath.Join(s.root, id+ext)
}

func (s *Storage) List() ([]string, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, fmt.Errorf("storage: list: %w", err)
	}

	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ext) {
			continue
		}
		ids = append(ids, strings.TrimSuffix(name, ext))
	}
	return ids, nil
}

type PendingFile struct {
	f     *os.File
	tmp   string
	path  string
	store *Storage
}

func (p *PendingFile) Write(b []byte) (int, error) {
	return p.f.Write(b)
}

func (p *PendingFile) Close() error {
	if err := p.f.Close(); err != nil {
		os.Remove(p.tmp)
		return fmt.Errorf("storage: close temp: %w", err)
	}
	if err := os.Rename(p.tmp, p.path); err != nil {
		os.Remove(p.tmp)
		return fmt.Errorf("storage: rename: %w", err)
	}

	return nil
}

func (p *PendingFile) Abort() {
	p.f.Close()
	os.Remove(p.tmp)
}

func (p *PendingFile) CommitAs(id string) error {
	p.path = p.store.path(id)
	return p.Close()
}

func (s *Storage) Create(id string) (*PendingFile, error) {
	p, err := s.CreatePending()
	if err != nil {
		return nil, err
	}
	p.path = s.path(id)
	return p, nil
}

func (s *Storage) CreatePending() (*PendingFile, error) {
	f, err := os.CreateTemp(s.root, "pending.*")
	if err != nil {
		return nil, fmt.Errorf("storage: create temp: %w", err)
	}
	return &PendingFile{f: f, tmp: f.Name(), store: s}, nil
}

func (s *Storage) Open(id string) (io.ReadCloser, error) {
	f, err := os.Open(s.path(id))
	if err != nil {
		return nil, fmt.Errorf("storage: open: %w", err)
	}
	return f, nil
}

func (s *Storage) Write(id string, data []byte) error {
	f, err := s.Create(id)
	if err != nil {
		return err
	}
	defer f.Abort()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("storage: write: %w", err)
	}
	return f.Close()
}

func (s *Storage) Read(id string) ([]byte, error) {
	data, err := os.ReadFile(s.path(id))
	if err != nil {
		return nil, fmt.Errorf("storage: read: %w", err)
	}
	return data, nil
}

func (s *Storage) Delete(id string) error {
	if err := os.Remove(s.path(id)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("storage: delete: %w", err)
	}
	return nil

}
