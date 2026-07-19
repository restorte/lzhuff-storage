package huffman

import "testing"

func TestBits(t *testing.T) {
	tests := []struct {
		name string
		bits []byte
	}{
		{name: "9 bits, crosses byte boundary", bits: []byte{1, 0, 1, 1, 0, 0, 0, 1, 1}},
		{name: "8 bits, exact byte", bits: []byte{1, 1, 0, 0, 1, 0, 1, 0}},
		{name: "16 bits, two exact bytes", bits: []byte{1, 0, 1, 0, 1, 0, 1, 0, 0, 1, 0, 1, 0, 1, 0, 1}},
		{name: "3 bits, padding only", bits: []byte{1, 0, 1}},
		{name: "1 bit", bits: []byte{1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &bitWriter{}
			for _, bit := range tt.bits {
				w.writeBit(bit)
			}
			w.flush()

			r := &bitReader{buf: w.buf}
			for i := 0; i < len(tt.bits); i++ {
				if got := r.readBit(); got != tt.bits[i] {
					t.Errorf("bit %d: got %d, want %d", i, got, tt.bits[i])
				}
			}
		})
	}
}
