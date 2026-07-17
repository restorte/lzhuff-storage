package lz77

import (
	"bytes"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	buf := make([]byte, 256)
	for i := range buf {
		buf[i] = byte(i)
	}
	tests := []struct {
		name string
		in   []byte
	}{
		{name: "empty", in: []byte{}},
		{name: "one byte", in: []byte("a")},
		{name: "eight bytes", in: []byte("abcdefgh")},
		{name: "nine bytes", in: []byte("abcdefghi")},
		{name: "abcabcabc", in: []byte("abcabcabc")},
		{name: "21 bytes", in: []byte("abcggtohabcmkikkruyvt")},
		{name: "repeated", in: bytes.Repeat([]byte("a"), 10000)},
		{name: "all bytes", in: buf},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compressed, err := Compress(tt.in)
			if err != nil {
				t.Fatalf("Compress(%v) returned unexpected error: %v", tt.in, err)
			}
			got, err := Decompress(compressed)
			if err != nil {
				t.Fatalf("Deсompress(%v) returned unexpected error: %v", tt.in, err)
			}
			if !bytes.Equal(got, tt.in) {
				t.Errorf("round-trip mismatch: in=%v, out=%v", tt.in, got)
			}
		})
	}
}

func TestToken(t *testing.T) {
	tests := []struct {
		name   string
		offset int
		lenght int
	}{
		{name: "offset = 300", offset: 300, lenght: 3},
		{name: "offset = 4095", offset: 4095, lenght: 18},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOFF, gotLEN := unpackToken(packToken(tt.offset, tt.lenght))
			if gotOFF != tt.offset {
				t.Errorf("gotOFF = %v, want = %v", gotOFF, tt.offset)
			}
			if gotLEN != tt.lenght {
				t.Errorf("gotLEN = %v, want = %v", gotLEN, tt.lenght)
			}
		})
	}
}
