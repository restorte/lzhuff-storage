package storage

import (
	"errors"
	"fmt"
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

func (s *Storage) Write(id string, data []byte) error {
	path := s.path(id)
	tmp := path + ".tmp"

	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("storage: write temp: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("storage: rename: %w", err)
	}

	return nil
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
