package lz77

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
	return nil, nil
}
