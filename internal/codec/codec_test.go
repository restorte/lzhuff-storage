package codec

import (
	"bytes"
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
