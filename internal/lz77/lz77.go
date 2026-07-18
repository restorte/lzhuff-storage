package lz77

const (
	WINDOW    = 4096
	MIN_MATCH = 3
	MAX_MATCH = 18
)

func findMatch(data []byte, i int) (offset, length int) {
	bestLen, bestOffset := 0, 0
	start := max(0, i-WINDOW)

	for j := start; j < i; j++ {
		k := 0
		for i+k < len(data) && k < MAX_MATCH && data[j+k] == data[i+k] {
			k += 1
		}
		if k > bestLen {
			bestLen = k
			bestOffset = i - j
		}
	}
	if bestLen >= MIN_MATCH {
		return bestOffset, bestLen
	}
	return 0, 0
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
	var result []byte
	var chunk []byte
	count, pos, control := 0, 0, 0

	for pos < len(data) {
		offset, length := findMatch(data, pos)

		if length > 0 {
			control |= 1 << count
			older, young := packToken(offset, length)
			chunk = append(chunk, older, young)
			pos += length
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
