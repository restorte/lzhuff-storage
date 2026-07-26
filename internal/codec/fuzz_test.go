package codec

import "testing"

func FuzzDecompress(f *testing.F) {
	valid, _ := Compress([]byte("hello hello hello world world"))
	f.Add(valid)
	f.Add(valid[:len(valid)/2])
	f.Add([]byte{})
	f.Add([]byte{1})
	f.Add([]byte{1, 0, 0, 0, 5})
	f.Add([]byte{0, 1, 2, 3})

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = Decompress(data)
	})
}
