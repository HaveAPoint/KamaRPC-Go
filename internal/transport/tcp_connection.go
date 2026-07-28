package transport

import (
	"bufio"
	"kamaRPC/internal/protocol"
	"net"
	"sync"
)

const BufferSize = 4096

// 包缓冲区（处理粘包）
type PacketBuffer struct {
	buf  []byte
	lock sync.Mutex
}

func (pb *PacketBuffer) Write(data []byte) {
	pb.lock.Lock()
	pb.buf = append(pb.buf, data...)
	pb.lock.Unlock()
}

func (pb *PacketBuffer) Read() ([]byte, error) {
	pb.lock.Lock()
	defer pb.lock.Unlock()

	// 固定包头还没收齐
	if len(pb.buf) < protocol.HeaderFixedLen {
		return nil, nil
	}

	headerLen := protocol.DecodeHeaderLen(pb.buf[2:6])
	bodyLen := protocol.DecodeBodyLen(pb.buf[6:10])

	// 关键：先校验再算 totalLen。长度非法时这条连接已经不可信，直接报错让上层断开
	if err := protocol.ValidateLen(headerLen, bodyLen); err != nil {
		return nil, err
	}

	totalLen := protocol.HeaderFixedLen + int(headerLen) + int(bodyLen)

	if len(pb.buf) < totalLen {
		return nil, nil
	}

	packet := make([]byte, totalLen)
	copy(packet, pb.buf[:totalLen])

	// 移动窗口
	pb.buf = pb.buf[totalLen:]
	return packet, nil
}

type TCPConnection struct {
	conn   net.Conn
	reader *bufio.Reader
	buffer *PacketBuffer

	writeMu sync.Mutex
}

// 创建连接
func NewTCPConnection(conn net.Conn) *TCPConnection {
	return &TCPConnection{
		conn:   conn,
		reader: bufio.NewReaderSize(conn, BufferSize),
		buffer: &PacketBuffer{
			buf: make([]byte, 0, BufferSize*2),
		},
	}
}

func (tc *TCPConnection) Read() (*protocol.Message, error) {
	for {
		// 尝试从缓冲区取完整包。长度非法时必须立刻返回，
		// 否则外层循环会继续往 pb.buf 里 append，缓冲区无限增长
		packet, err := tc.buffer.Read()
		if err != nil {
			return nil, err
		}
		if packet != nil {
			return protocol.Decode(packet)
		}

		tmp := make([]byte, BufferSize)
		n, err := tc.reader.Read(tmp)
		if err != nil {
			return nil, err
		}

		if n > 0 {
			tc.buffer.Write(tmp[:n])
		}
	}
}

func (tc *TCPConnection) Write(msg *protocol.Message) error {
	data, err := protocol.Encode(msg)
	if err != nil {
		return err
	}

	tc.writeMu.Lock()
	defer tc.writeMu.Unlock()

	total := 0
	for total < len(data) {
		n, err := tc.conn.Write(data[total:])
		if err != nil {
			return err
		}
		total += n
	}

	return nil
}

// 关闭连接
func (tc *TCPConnection) Close() error {
	if tcp, ok := tc.conn.(*net.TCPConn); ok {
		tcp.SetLinger(0)
	}
	return tc.conn.Close()
}

func (tc *TCPConnection) RemoteAddr() string {
	return tc.conn.RemoteAddr().String()
}
