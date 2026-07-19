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

func TestFindMatch(t *testing.T) {
	tests := []struct {
		name    string
		in      []byte
		i       int
		wantLEN int
		wantOFF int
	}{
		{name: "zero matches", in: []byte("abcdtyujh"), i: 3, wantLEN: 0, wantOFF: 0},
		{name: "i = 0", in: []byte("asdfga"), i: 0, wantLEN: 0, wantOFF: 0},
		{name: "pure coincidence", in: []byte("zxczxc"), i: 3, wantLEN: 3, wantOFF: 3},
		{name: "overlap (run)", in: []byte("aaaaaaa"), i: 1, wantLEN: 6, wantOFF: 1},
		{name: "MAX_MATCH ceiling", in: []byte(bytes.Repeat([]byte("a"), 30)), i: 1, wantLEN: 18, wantOFF: 1},
		{name: "shorter than the threshold", in: []byte("abcab"), i: 3, wantLEN: 0, wantOFF: 0},
		{name: "lazy defer", in: []byte("ABCxBCDEFGyABCDEFG")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			head := make([]int, HASH_SIZE)

			for i := range head {
				head[i] = -1
			}
			prev := make([]int, len(tt.in))

			for p := 0; p < tt.i; p++ {
				insert(head, prev, tt.in, p)
			}
			gotOFF, gotLEN := findMatch(tt.in, tt.i, head, prev)

			if gotLEN != tt.wantLEN {
				t.Errorf("gotLEN = %v, want = %v", gotLEN, tt.wantLEN)
			}

			if gotOFF != tt.wantOFF {
				t.Errorf("gotOFF = %v, want = %v", gotOFF, tt.wantOFF)
			}
		})
	}
}

func TestHash(t *testing.T) {
	for a := 0; a < 256; a++ {
		for b := 0; b < 256; b++ {
			for c := 0; c < 256; c++ {
				h := hash(a, b, c)
				if h < 0 || h >= HASH_SIZE {
					t.Fatalf("hash(%d,%d,%d) = %d вне диапазона", a, b, c, h)

				}
			}
		}
	}
}
