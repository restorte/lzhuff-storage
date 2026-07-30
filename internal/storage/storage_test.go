package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"testing"
)

func TestStorage_WriteRead(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	data := []byte("arbitrary bytes, even binary \x00\xff\x10")
	id := "test-id"

	if err := s.Write(id, data); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := s.Read(id)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("Read = %q, want %q", got, data)
	}

	newData := []byte("replaced")
	if err := s.Write(id, newData); err != nil {
		t.Fatalf("Write (overwrite): %v", err)
	}
	got, err = s.Read(id)
	if err != nil {
		t.Fatalf("Read after overwrite: %v", err)
	}
	if !bytes.Equal(got, newData) {
		t.Errorf("after overwrite Read = %q, want %q", got, newData)
	}

	if err := s.Delete(id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Read(id); err == nil {
		t.Error("expected read error after Delete, got nil")
	}
}

func TestStorage_List(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"alpha", "beta"} {
		if err := s.Write(id, []byte("x")); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(dir, "gamma"+ext+".tmp"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	sort.Strings(got)

	want := []string{"alpha", "beta"}
	if !slices.Equal(got, want) {
		t.Errorf("List = %v, want %v", got, want)
	}
}

func TestStorage_DeleteMissingIsNoError(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("never-existed"); err != nil {
		t.Errorf("Delete of missing file returned error: %v", err)
	}
}
