package storage

import (
	"bytes"
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

func TestStorage_DeleteMissingIsNoError(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("never-existed"); err != nil {
		t.Errorf("Delete of missing file returned error: %v", err)
	}
}
