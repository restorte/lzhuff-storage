package huffman

type bitWriter struct {
	buf   []byte
	cur   byte
	nbits int
}

type bitReader struct {
	buf     []byte
	bytePos int
	bitPos  int
}

func (b *bitWriter) writeBit(bit byte) {
	b.cur = (b.cur << 1) | bit
	b.nbits++
	if b.nbits == 8 {
		b.buf = append(b.buf, b.cur)
		b.cur, b.nbits = 0, 0
	}
}

func (b *bitWriter) flush() {
	if b.nbits > 0 {
		b.cur <<= (8 - b.nbits)
		b.buf = append(b.buf, b.cur)
		b.cur, b.nbits = 0, 0
	}
}

func (b *bitReader) readBit() byte {
	bit := (b.buf[b.bytePos] >> (7 - b.bitPos) & 1)
	b.bitPos++
	if b.bitPos == 8 {
		b.bitPos = 0
		b.bytePos++
	}
	return bit
}
