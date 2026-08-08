package lz4

import (
	"bytes"

	"github.com/pierrec/lz4/v4"
)

func Compress(src []byte) []byte {
	var buf bytes.Buffer
	writer := lz4.NewWriter(&buf)
	writer.Write(src)
	writer.Close()
	return buf.Bytes()
}

func Decompress(src []byte, dstSize int) []byte {
	dst := make([]byte, dstSize)
	reader := lz4.NewReader(bytes.NewReader(src))
	reader.Read(dst)
	return dst
}
