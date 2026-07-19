package huffman

import "container/heap"

type node struct {
	char  byte
	freq  int
	left  *node
	right *node
}

type minHeap []*node

func (h minHeap) Len() int {
	return len(h)
}
func (h minHeap) Less(i, j int) bool {
	return h[i].freq < h[j].freq
}
func (h minHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *minHeap) Push(x interface{}) {
	*h = append(*h, x.(*node))
}

func (h *minHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

func countFreq(data []byte) [256]int {
	freq := [256]int{}
	for _, b := range data {
		freq[b]++
	}
	return freq
}

func buildHuffmanTree(freq [256]int) *node {
	h := &minHeap{}
	heap.Init(h)
	for b := 0; b < 256; b++ {
		if freq[b] > 0 {
			heap.Push(h, &node{char: byte(b), freq: freq[b]})
		}
	}
	for h.Len() > 1 {
		left := heap.Pop(h).(*node)
		right := heap.Pop(h).(*node)
		merged := &node{
			freq:  left.freq + right.freq,
			left:  left,
			right: right,
		}
		heap.Push(h, merged)
	}
	return heap.Pop(h).(*node)
}

func buildCodes(root *node, prefix string, codes map[byte]string) {
	if root == nil {
		return
	}
	if root.left == nil && root.right == nil {
		codes[root.char] = prefix
		return
	}
	buildCodes(root.left, prefix+"0", codes)
	buildCodes(root.right, prefix+"1", codes)
}

func encode(data []byte, codes map[byte]string, w *bitWriter) {
	for _, b := range data {
		code := codes[b]
		for i := range code {
			w.writeBit(code[i] - '0')
		}
	}
}

func decode(r *bitReader, root *node, n int) []byte {
	res := make([]byte, 0, n)
	if root.left == nil && root.right == nil {
		for len(res) < n {
			res = append(res, root.char)
		}
		return res
	}
	cur := root
	for len(res) < n {
		if r.readBit() == 0 {
			cur = cur.left
		} else {
			cur = cur.right
		}
		if cur.left == nil && cur.right == nil {
			res = append(res, cur.char)
			cur = root
		}
	}
	return res
}

func (w *bitWriter) writeByte(b byte) {
	for i := 7; i >= 0; i-- {
		w.writeBit((b >> i) & 1)
	}
}

func (r *bitReader) readByte() byte {
	var b byte
	for i := 0; i < 8; i++ {
		b = (b << 1) | r.readBit()
	}
	return b
}

func writeTree(n *node, w *bitWriter) {
	if n.left == nil && n.right == nil {
		w.writeBit(1)
		w.writeByte(n.char)
		return
	}
	w.writeBit(0)
	writeTree(n.left, w)
	writeTree(n.right, w)
}

func readTree(r *bitReader) *node {
	if r.readBit() == 1 {
		return &node{char: r.readByte()}
	}
	left := readTree(r)
	right := readTree(r)
	return &node{left: left, right: right}
}

func Compress(data []byte) []byte {
	w := &bitWriter{}
	n := len(data)
	w.writeByte(byte(n >> 24))
	w.writeByte(byte(n >> 16))
	w.writeByte(byte(n >> 8))
	w.writeByte(byte(n))
	if n == 0 {
		w.flush()
		return w.buf
	}
	root := buildHuffmanTree(countFreq(data))
	codes := map[byte]string{}
	buildCodes(root, "", codes)
	writeTree(root, w)
	encode(data, codes, w)
	w.flush()
	return w.buf
}

func Decompress(data []byte) []byte {
	r := &bitReader{buf: data}
	b0 := int(r.readByte())
	b1 := int(r.readByte())
	b2 := int(r.readByte())
	b3 := int(r.readByte())
	n := b0<<24 | b1<<16 | b2<<8 | b3
	if n == 0 {
		return []byte{}
	}
	root := readTree(r)
	return decode(r, root, n)
}
