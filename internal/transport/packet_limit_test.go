package transport

import (
	"encoding/binary"
	"errors"
	"kamaRPC/internal/protocol"
	"net"
	"testing"
)

// makeHead 构造一个只有固定包头、不带实际 header/body 的字节切片
func makeHead(headerLen, bodyLen uint32) []byte {
	head := make([]byte, protocol.HeaderFixedLen)
	binary.BigEndian.PutUint16(head[0:2], protocol.Magic)
	binary.BigEndian.PutUint32(head[2:6], headerLen)
	binary.BigEndian.PutUint32(head[6:10], bodyLen)
	return head
}

// TestPacketBuffer_RejectsOversizedLen 回归：对端声明超大长度曾可让 pb.buf 无限增长
func TestPacketBuffer_RejectsOversizedLen(t *testing.T) {
	tests := []struct {
		name      string
		headerLen uint32
		bodyLen   uint32
		wantErr   error
	}{
		{"body 声明 1GB", 20, 1 << 30, protocol.ErrBodyTooLarge},
		{"body 声明 uint32 最大值", 20, 0xFFFFFFFF, protocol.ErrBodyTooLarge},
		{"header 声明 4MB", 4 << 20, 0, protocol.ErrHeaderTooLarge},
		{"两者都超限，先报 header", 4 << 20, 1 << 30, protocol.ErrHeaderTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb := &PacketBuffer{}
			pb.Write(makeHead(tt.headerLen, tt.bodyLen))

			packet, err := pb.Read()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Read() error = %v, want %v", err, tt.wantErr)
			}
			if packet != nil {
				t.Fatalf("Read() packet = %d bytes, want nil", len(packet))
			}
		})
	}
}

// TestPacketBuffer_ReadWaitsForCompletePacket 合法长度但数据未收齐时应返回 (nil, nil)
func TestPacketBuffer_ReadWaitsForCompletePacket(t *testing.T) {
	pb := &PacketBuffer{}
	pb.Write(makeHead(20, 100))

	packet, err := pb.Read()
	if err != nil {
		t.Fatalf("Read() error = %v, want nil", err)
	}
	if packet != nil {
		t.Fatalf("Read() packet = %d bytes, want nil", len(packet))
	}
}

// TestTCPConnection_ReadRejectsOversizedLen 校验错误要一路传到 TCPConnection.Read，
// 让上层关掉连接，而不是继续从 socket 读并无限缓冲。
func TestTCPConnection_ReadRejectsOversizedLen(t *testing.T) {
	local, remote := net.Pipe()
	defer local.Close()
	defer remote.Close()

	conn := NewTCPConnection(local)

	go func() {
		// 声明 1GB body 后不再发任何数据：
		// 修复前 Read 会永久阻塞并持续增长 pb.buf
		remote.Write(makeHead(20, 1<<30))
	}()

	msg, err := conn.Read()
	if !errors.Is(err, protocol.ErrBodyTooLarge) {
		t.Fatalf("Read() error = %v, want %v", err, protocol.ErrBodyTooLarge)
	}
	if msg != nil {
		t.Fatalf("Read() msg = %v, want nil", msg)
	}
}
