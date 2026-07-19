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

func encode(data []byte, codes map[byte]string) string {
	res := ""
	for _, b := range data {
		res += codes[b]
	}
	return res
}

func decode(encoded string, root *node) []byte {
	res := []byte{}
	cur := root
	for _, ch := range encoded {
		if ch == '0' {
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
