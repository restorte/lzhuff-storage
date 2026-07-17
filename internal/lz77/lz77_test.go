package lz77

import (
	"bytes"
	"testing"
)

func TestCompressV0(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want []byte
	}{
		{name: "empty", in: []byte{}, want: []byte{}},
		{name: "one byte", in: []byte("a"), want: []byte{0, 'a'}},
		{name: "eight bytes", in: []byte("abcdefgh"), want: []byte{0, 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'}},
		{name: "nine bytes", in: []byte("abcdefghi"), want: []byte{0, 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 0, 'i'}},
		{name: "abcabcabc", in: []byte("abcabcabc"), want: []byte{0, 'a', 'b', 'c', 'a', 'b', 'c', 'a', 'b', 0, 'c'}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Compress(tt.in)
			if err != nil {
				t.Fatalf("Compress(%v) returned unexpected error: %v", tt.in, err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("Copmress(%v) = %v, want %v", tt.in, got, tt.want)
			}

		})
	}
}
