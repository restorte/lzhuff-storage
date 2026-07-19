package codec

import (
	"github.com/restorte/lzhuff-store/internal/huffman"
	"github.com/restorte/lzhuff-store/internal/lz77"
)

func Compress(data []byte) ([]byte, error) {
	lzOut, err := lz77.Compress(data)
	if err != nil {
		return nil, err
	}
	return huffman.Compress(lzOut), nil
}

func Decompress(data []byte) ([]byte, error) {
	lzStream := huffman.Decompress(data)
	return lz77.Decompress(lzStream)
}
