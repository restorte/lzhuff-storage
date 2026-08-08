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

func blocksFor(n int) int {
	return (n + blockSize - 1) / blockSize
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
	limit := len(raw) + headerSize + blockHeaderSize*blocksFor(len(raw))
	if len(comp) > limit {
		t.Errorf("compressed random data to %d bytes, want at most %d", len(comp), limit)
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
	if flag := comp[headerSize+4]; flag != flagCompressed {
		t.Fatalf("first block flag = %d, want %d (compressible data must not be stored raw)", flag, flagCompressed)
	}
	if len(comp) >= len(raw)/2 {
		t.Errorf("compressed %d bytes to %d, expected well under half", len(raw), len(comp))
	}
}

func TestRoundTripAcrossBlockBoundaries(t *testing.T) {
	sizes := []int{
		blockSize - 1,
		blockSize,
		blockSize + 1,
		2*blockSize + 12345,
	}
	for _, n := range sizes {
		raw := make([]byte, n)
		if _, err := rand.Read(raw[:n/2]); err != nil {
			t.Fatal(err)
		}
		copy(raw[n/2:], bytes.Repeat([]byte("abcabcabc"), n))

		comp, err := Compress(raw)
		if err != nil {
			t.Fatalf("%d bytes: Compress: %v", n, err)
		}
		back, err := Decompress(comp)
		if err != nil {
			t.Fatalf("%d bytes: Decompress: %v", n, err)
		}
		if !bytes.Equal(back, raw) {
			t.Errorf("%d bytes: round-trip mismatch", n)
		}
	}
}

func TestStreamMatchesBuffered(t *testing.T) {
	raw := bytes.Repeat([]byte("streaming and buffered must agree "), 5000)

	var streamed bytes.Buffer
	if err := CompressStream(bytes.NewReader(raw), &streamed); err != nil {
		t.Fatalf("CompressStream: %v", err)
	}
	buffered, err := Compress(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(streamed.Bytes(), buffered) {
		t.Error("CompressStream and Compress produced different containers")
	}

	var back bytes.Buffer
	if err := DecompressStream(bytes.NewReader(buffered), &back); err != nil {
		t.Fatalf("DecompressStream: %v", err)
	}
	if !bytes.Equal(back.Bytes(), raw) {
		t.Error("DecompressStream did not reproduce the input")
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

func TestDecompressSurvivesTruncation(t *testing.T) {
	full, err := Compress([]byte("hello hello hello world world world"))
	if err != nil {
		t.Fatal(err)
	}

	for i := range full {
		_, _ = Decompress(full[:i])
	}
}

func TestDecompressRejectsAbsurdLength(t *testing.T) {
	bogus := []byte{1, 0x7F, 0xFF, 0xFF, 0xFF}
	if _, err := Decompress(bogus); err == nil {
		t.Error("expected an error for an impossible declared length")
	}
}
