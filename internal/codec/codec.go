package codec

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/restorte/lzhuff-storage/internal/huffman"
	"github.com/restorte/lzhuff-storage/internal/lz77"
)

const (
	magic     = "LZH1"
	blockSize = 256 << 10

	maxBlockSize = 16 << 20

	headerSize      = len(magic) + 4
	blockHeaderSize = 5

	flagStored     byte = 0
	flagCompressed byte = 1
)

func CompressStream(r io.Reader, w io.Writer) error {
	if _, err := io.WriteString(w, magic); err != nil {
		return fmt.Errorf("codec: write magic: %w", err)
	}
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], blockSize)
	if _, err := w.Write(size[:]); err != nil {
		return fmt.Errorf("codec: write block size: %w", err)
	}

	buf := make([]byte, blockSize)
	for {
		n, err := io.ReadFull(r, buf)
		if n > 0 {
			if werr := writeBlock(w, buf[:n]); werr != nil {
				return werr
			}
		}
		switch {
		case err == nil:
			continue
		case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
			return nil
		default:
			return fmt.Errorf("codec: read: %w", err)
		}
	}
}

func DecompressStream(r io.Reader, w io.Writer) error {
	var head [headerSize]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return fmt.Errorf("codec: truncated container header: %w", err)
	}
	if string(head[:len(magic)]) != magic {
		return errors.New("codec: not a container (bad magic)")
	}
	declared := binary.BigEndian.Uint32(head[len(magic):])
	if declared == 0 || declared > maxBlockSize {
		return fmt.Errorf("codec: implausible block size %d", declared)
	}

	var buf []byte
	for {
		var bh [blockHeaderSize]byte
		if _, err := io.ReadFull(r, bh[:]); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("codec: truncated block header: %w", err)
		}

		n := binary.BigEndian.Uint32(bh[:4])
		flag := bh[4]
		if n > declared {
			return fmt.Errorf("codec: block of %d exceeds the declared size %d", n, declared)
		}

		if cap(buf) < int(n) {
			buf = make([]byte, n)
		}
		block := buf[:n]
		if _, err := io.ReadFull(r, block); err != nil {
			return fmt.Errorf("codec: truncated block: %w", err)
		}

		switch flag {
		case flagStored:
			if _, err := w.Write(block); err != nil {
				return fmt.Errorf("codec: write: %w", err)
			}
		case flagCompressed:
			lzStream, err := huffman.Decompress(block)
			if err != nil {
				return err
			}
			raw, err := lz77.Decompress(lzStream)
			if err != nil {
				return err
			}
			if _, err := w.Write(raw); err != nil {
				return fmt.Errorf("codec: write: %w", err)
			}
		default:
			return fmt.Errorf("codec: unknown block flag %d", flag)
		}
	}
}

func writeBlock(w io.Writer, raw []byte) error {
	lzOut, err := lz77.Compress(raw)
	if err != nil {
		return err
	}
	comp := huffman.Compress(lzOut)

	flag, payload := flagCompressed, comp
	if len(comp) >= len(raw) {
		flag, payload = flagStored, raw
	}

	var bh [blockHeaderSize]byte
	binary.BigEndian.PutUint32(bh[:4], uint32(len(payload)))
	bh[4] = flag
	if _, err := w.Write(bh[:]); err != nil {
		return fmt.Errorf("codec: write block header: %w", err)
	}
	if _, err := w.Write(payload); err != nil {
		return fmt.Errorf("codec: write block: %w", err)
	}
	return nil
}

func Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := CompressStream(bytes.NewReader(data), &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func Decompress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := DecompressStream(bytes.NewReader(data), &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
