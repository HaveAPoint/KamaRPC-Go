package codec

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"sync"
)

// CompressionType 压缩类型
type CompressionType byte

const (
	CompressionNone CompressionType = iota
	CompressionGzip
)

// MaxDecompressedSize 解压后的上限。压缩比可达 1000:1，
// 一个通过了 MaxBodySize 检查的小包解压后仍可能撑爆内存。
const MaxDecompressedSize = 8 << 20 // 8MB

var ErrDecompressedTooLarge = fmt.Errorf("decompressed data exceeds %d bytes", MaxDecompressedSize)

// Compressor 压缩接口
type compressor interface {
	compress([]byte) ([]byte, error)
	decompress([]byte) ([]byte, error)
}

// GzipCompressor gzip 压缩器
type GzipCompressor struct{}

func (g *GzipCompressor) compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, err := w.Write(data)
	if err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (g *GzipCompressor) decompress(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()

	// 多读 1 字节：能读到说明超限了。不能用 ReadAll，
	// 读多少完全由压缩包内容决定，等于让对端决定我们分配多少内存
	var buf bytes.Buffer
	n, err := io.CopyN(&buf, r, MaxDecompressedSize+1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if n > MaxDecompressedSize {
		return nil, ErrDecompressedTooLarge
	}

	return buf.Bytes(), nil
}

var (
	gzipCompressor = &GzipCompressor{}
	compressorMu   sync.RWMutex
	compressors    = make(map[CompressionType]compressor)
)

// RegisterCompressor 注册压缩器
func RegisterCompressor(t CompressionType, c compressor) {
	compressorMu.Lock()
	defer compressorMu.Unlock()
	compressors[t] = c
}

// GetCompressor 获取压缩器
func GetCompressor(t CompressionType) compressor {
	compressorMu.RLock()
	defer compressorMu.RUnlock()
	return compressors[t]
}

func init() {
	RegisterCompressor(CompressionGzip, gzipCompressor)
}

// Compress 使用指定类型压缩
func Compress(data []byte, t CompressionType) ([]byte, error) {
	c := GetCompressor(t)
	if c == nil {
		return nil, errors.New("compressor not found")
	}
	return c.compress(data)
}

// Decompress 使用指定类型解压
func Decompress(data []byte, t CompressionType) ([]byte, error) {
	c := GetCompressor(t)
	if c == nil {
		return nil, errors.New("compressor not found")
	}
	return c.decompress(data)
}
