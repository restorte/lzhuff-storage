package codec

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestRoundTripLZHUFF(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
	}{
		{name: "empty", in: []byte("")},
		{name: "single", in: []byte("aaaa")},
		{name: "text", in: []byte("the quick brown fox jumps over the lazy dog")},
		{name: "abcabc", in: bytes.Repeat([]byte("abc"), 1000)},
		{name: "repeated", in: bytes.Repeat([]byte("a"), 10000)},
		{name: "all bytes", in: func() []byte {
			b := make([]byte, 256)
			for i := range b {
				b[i] = byte(i)
			}
			return b
		}()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comp, err := Compress(tt.in)
			if err != nil {
				t.Fatalf("Compress: %v", err)
			}
			back, err := Decompress(comp)
			if err != nil {
				t.Fatalf("Decompress: %v", err)

			}
			if !bytes.Equal(tt.in, back) {
				t.Errorf("round-trip mismatch: in=%q out=%q", tt.in, back)

			}
		})
	}

}

func TestIncompressibleNeverGrows(t *testing.T) {
	raw := make([]byte, 10000)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}

	comp, err := Compress(raw)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}
	if len(comp) > len(raw)+1 {
		t.Errorf("compressed random data to %d bytes, want at most %d", len(comp), len(raw)+1)
	}

	back, err := Decompress(comp)
	if err != nil {
		t.Fatalf("Decompress: %v", err)
	}
	if !bytes.Equal(back, raw) {
		t.Error("round-trip mismatch on stored (uncompressed) container")
	}
}

func TestCompressibleStillShrinks(t *testing.T) {
	raw := bytes.Repeat([]byte("the quick brown fox "), 500)

	comp, err := Compress(raw)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}
	if comp[0] != flagCompressed {
		t.Fatalf("flag = %d, want %d (compressible data must not be stored raw)", comp[0], flagCompressed)
	}
	if len(comp) >= len(raw)/2 {
		t.Errorf("compressed %d bytes to %d, expected well under half", len(raw), len(comp))
	}
}

func TestDecompressRejectsBadContainer(t *testing.T) {
	if _, err := Decompress(nil); err == nil {
		t.Error("empty container: expected an error, got nil")
	}
	if _, err := Decompress([]byte{9, 1, 2, 3}); err == nil {
		t.Error("unknown flag: expected an error, got nil")
	}
}
