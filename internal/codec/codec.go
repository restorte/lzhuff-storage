package codec

import (
	"errors"
	"fmt"

	"github.com/restorte/lzhuff-store/internal/huffman"
	"github.com/restorte/lzhuff-store/internal/lz77"
)

const (
	flagStored     byte = 0
	flagCompressed byte = 1
)

var ErrEmptyContainer = errors.New("codec: empty container")

func Compress(data []byte) ([]byte, error) {
	lzOut, err := lz77.Compress(data)
	if err != nil {
		return nil, err
	}
	comp := huffman.Compress(lzOut)

	if len(comp) >= len(data) {
		return append([]byte{flagStored}, data...), nil
	}
	return append([]byte{flagCompressed}, comp...), nil
}

func Decompress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrEmptyContainer
	}

	switch data[0] {
	case flagStored:
		return data[1:], nil
	case flagCompressed:
		return lz77.Decompress(huffman.Decompress(data[1:]))
	default:
		return nil, fmt.Errorf("codec: unknown container flag %d", data[0])
	}
}
