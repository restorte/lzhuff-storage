package huffman

import (
	"bytes"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
	}{
		{name: "abracadabra", in: []byte("abracadabra")},
		{name: "mississippi", in: []byte("mississippi")},
		{name: "hello world", in: []byte("hello world")},
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
			freq := countFreq(tt.in)
			root := buildHuffmanTree(freq)
			codes := map[byte]string{}
			buildCodes(root, "", codes)
			back := decode(encode(tt.in, codes), root)

			if !bytes.Equal(back, tt.in) {
				t.Errorf("round-trip mismatch: in=%q, out=%q", tt.in, back)

			}
		})
	}
}
