package huffman

import (
	"bytes"
	"reflect"
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
		{name: "empty", in: []byte("")},
		{name: "single symbol", in: []byte("aaaa")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			back := Decompress(Compress(tt.in))
			if !bytes.Equal(back, tt.in) {
				t.Errorf("round-trip mismatch: in=%q, out=%q", tt.in, back)
			}
		})
	}
}

func TestTreeSerialize(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
	}{
		{name: "abracadabra", in: []byte("abracadabra")},
		{name: "mississippi", in: []byte("mississippi")},
		{name: "hello world", in: []byte("hello world")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := buildHuffmanTree(countFreq(tt.in))

			w := &bitWriter{}
			writeTree(root, w)
			w.flush()

			root2 := readTree(&bitReader{buf: w.buf})

			codes1 := map[byte]string{}
			buildCodes(root, "", codes1)
			codes2 := map[byte]string{}
			buildCodes(root2, "", codes2)

			if !reflect.DeepEqual(codes1, codes2) {
				t.Errorf("codes mismatch after serialize: before=%v after=%v", codes1, codes2)
			}
		})
	}
}
