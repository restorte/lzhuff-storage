package lz77

const (
	WINDOW    = 4096
	MIN_MATCH = 3
	MAX_MATCH = 18
	HASH_BITS = 15
	HASH_SIZE = 32768
	HASH_MASK = HASH_SIZE - 1
	MAX_CHAIN = 256
)

func hash(a, b, c int) int {
	h := (a<<10 ^ b<<5 ^ c) & HASH_MASK
	return h
}

func findMatch(data []byte, i int, head, prev []int) (offset, length int) {
	bestLen, bestOffset := 0, 0
	start := max(0, i-WINDOW)
	if i+2 >= len(data) {
		return 0, 0
	}
	j := head[hash(int(data[i]), int(data[i+1]), int(data[i+2]))]
	chain := 0
	for j >= start && chain < MAX_CHAIN {
		k := 0
		for i+k < len(data) && k < MAX_MATCH && data[j+k] == data[i+k] {
			k += 1
		}
		if k > bestLen {
			bestLen = k
			bestOffset = i - j
		}
		if bestLen >= MAX_MATCH {
			break
		}
		j = prev[j]
		chain += 1
	}
	if bestLen >= MIN_MATCH {
		return bestOffset, bestLen
	}
	return 0, 0
}

func insert(head, prev []int, data []byte, p int) {
	if p+2 >= len(data) {
		return
	}
	h := hash(int(data[p]), int(data[p+1]), int(data[p+2]))
	prev[p] = head[h]
	head[h] = p
}

func packToken(offset int, length int) (older, young byte) {
	length -= MIN_MATCH
	token := ((offset - 1) << 4) | length
	older = byte(token >> 8)
	young = byte(token & 0xFF)

	return older, young
}

func unpackToken(older, young byte) (offset int, length int) {
	token := int(older)<<8 | int(young)
	offset = token >> 4
	length = (token & 0x0F) + MIN_MATCH

	return offset + 1, length

}

func Compress(data []byte) ([]byte, error) {
	head := make([]int, HASH_SIZE)
	prev := make([]int, len(data))
	var result []byte
	var chunk []byte
	count, pos, control := 0, 0, 0

	for i := range head {
		head[i] = -1
	}

	for pos < len(data) {
		offset0, length0 := findMatch(data, pos, head, prev)
		insert(head, prev, data, pos)

		if length0 > 0 {
			_, length1 := findMatch(data, pos+1, head, prev)
			if length1 > length0 {
				chunk = append(chunk, data[pos])
				pos += 1
			} else {
				control |= 1 << count
				older, young := packToken(offset0, length0)
				chunk = append(chunk, older, young)

				for p := pos + 1; p < pos+length0; p++ {
					insert(head, prev, data, p)
				}
				pos += length0
			}
		} else {
			chunk = append(chunk, data[pos])
			pos += 1
		}
		count++
		if count == 8 {
			result = append(result, byte(control))
			result = append(result, chunk...)
			control, chunk, count = 0, nil, 0
		}

	}
	if count > 0 {
		result = append(result, byte(control))
		result = append(result, chunk...)
	}
	return result, nil
}

func Decompress(data []byte) ([]byte, error) {
	var result []byte
	pos := 0

	for pos < len(data) {
		control := data[pos]
		pos++
		for i := 0; i < 8; i++ {
			if pos >= len(data) {
				break
			}
			if control&(1<<i) != 0 {
				offset, length := unpackToken(data[pos], data[pos+1])
				pos += 2
				srcStart := len(result) - offset
				for k := 0; k < length; k++ {
					result = append(result, result[srcStart+k])
				}
			} else {
				result = append(result, data[pos])
				pos++
			}
		}
	}
	return result, nil
}
