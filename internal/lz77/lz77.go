package lz77

const (
	WINDOW    = 4096
	MIN_MATCH = 3
	MAX_MATCH = 18
)

func packToken(offset int, length int) (older, young byte) {
	length -= MIN_MATCH
	token := (offset << 4) | length
	older = byte(token >> 8)
	young = byte(token & 0xFF)

	return older, young
}

func unpackToken(older, young byte) (offset int, length int) {
	token := int(older)<<8 | int(young)
	offset = token >> 4
	length = (token & 0x0F) + MIN_MATCH

	return offset, length

}

func Compress(data []byte) ([]byte, error) {

	var result []byte

	for i := 0; i < len(data); i += 8 {
		end := i + 8

		if end > len(data) {

			end = len(data)

		}
		result = append(result, 0)
		result = append(result, data[i:end]...)

	}

	return result, nil

}

func Decompress(data []byte) ([]byte, error) {
	var result []byte
	pos := 0

	for pos < len(data) {
		pos += 1
		end := min(pos+8, len(data))
		result = append(result, data[pos:end]...)
		pos = end
	}
	return result, nil
}
