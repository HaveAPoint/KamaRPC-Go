package codec

import (
	"bytes"
	"errors"
	"testing"
)

// TestDecompress_RejectsBomb 回归：io.ReadAll 解压曾无上限，
// 小压缩包可展开成巨量数据
func TestDecompress_RejectsBomb(t *testing.T) {
	// 全零数据压缩比极高，是最简单的解压炸弹
	original := make([]byte, MaxDecompressedSize*4)

	bomb, err := Compress(original, CompressionGzip)
	if err != nil {
		t.Fatalf("Compress() error = %v", err)
	}

	t.Logf("原始 %d 字节压缩为 %d 字节，压缩比 %.0f:1",
		len(original), len(bomb), float64(len(original))/float64(len(bomb)))

	// 关键前提：压缩包本身很小，能通过 protocol.MaxBodySize 的检查
	if len(bomb) >= MaxDecompressedSize {
		t.Fatalf("测试前提不成立：压缩包 %d 字节不够小", len(bomb))
	}

	got, err := Decompress(bomb, CompressionGzip)
	if !errors.Is(err, ErrDecompressedTooLarge) {
		t.Fatalf("Decompress() error = %v, want %v", err, ErrDecompressedTooLarge)
	}
	if got != nil {
		t.Fatalf("Decompress() returned %d bytes, want nil", len(got))
	}
}

// TestDecompress_RoundTrip 正常大小的数据必须能往返
func TestDecompress_RoundTrip(t *testing.T) {
	original := []byte(`{"service":"Arith","method":"Add","a":1,"b":2}`)

	compressed, err := Compress(original, CompressionGzip)
	if err != nil {
		t.Fatalf("Compress() error = %v", err)
	}

	got, err := Decompress(compressed, CompressionGzip)
	if err != nil {
		t.Fatalf("Decompress() error = %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("Decompress() = %q, want %q", got, original)
	}
}

// TestDecompress_AtLimit 边界值：正好等于上限应通过
func TestDecompress_AtLimit(t *testing.T) {
	original := make([]byte, MaxDecompressedSize)

	compressed, err := Compress(original, CompressionGzip)
	if err != nil {
		t.Fatalf("Compress() error = %v", err)
	}

	got, err := Decompress(compressed, CompressionGzip)
	if err != nil {
		t.Fatalf("Decompress() error = %v, want nil", err)
	}
	if len(got) != MaxDecompressedSize {
		t.Fatalf("len(Decompress()) = %d, want %d", len(got), MaxDecompressedSize)
	}
}

// TestDecompress_CorruptedData gzip 流损坏要报校验错误，不能和超限混淆
func TestDecompress_CorruptedData(t *testing.T) {
	compressed, err := Compress([]byte("hello"), CompressionGzip)
	if err != nil {
		t.Fatalf("Compress() error = %v", err)
	}

	// 破坏最后一个字节（CRC 校验区）
	compressed[len(compressed)-1] ^= 0xFF

	_, err = Decompress(compressed, CompressionGzip)
	if err == nil {
		t.Fatal("Decompress() error = nil, want checksum error")
	}
	if errors.Is(err, ErrDecompressedTooLarge) {
		t.Fatalf("Decompress() error = %v, want checksum error not size error", err)
	}
}
