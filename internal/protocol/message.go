package protocol

import (
	"encoding/binary"
	"fmt"
	"kamaRPC/internal/codec"
)

const Magic uint16 = 0x1234

// HeaderFixedLen 固定包头长度：magic(2) + headerLen(4) + bodyLen(4)
const HeaderFixedLen = 10

// 单包长度上限。远端声明的长度先校验再分配，否则一个连接就能耗尽内存。
const (
	MaxHeaderSize = 1 << 16 // 64KB，header 只放服务名/方法名/错误串
	MaxBodySize   = 4 << 20 // 4MB
)

var (
	ErrHeaderTooLarge = fmt.Errorf("header exceeds %d bytes", MaxHeaderSize)
	ErrBodyTooLarge   = fmt.Errorf("body exceeds %d bytes", MaxBodySize)
)

// ValidateLen 校验对端声明的长度是否在允许范围内。
// 必须在用这两个值做任何分配或切片之前调用。
func ValidateLen(headerLen, bodyLen uint32) error {
	if headerLen > MaxHeaderSize {
		return ErrHeaderTooLarge
	}
	if bodyLen > MaxBodySize {
		return ErrBodyTooLarge
	}
	return nil
}

type Message struct {
	Header *Header
	Body   []byte
}

func Encode(msg *Message) ([]byte, error) {

	if msg.Header == nil {
		return nil, fmt.Errorf("header is nil")
	}

	bodyBytes := msg.Body

	if msg.Header.Compression != codec.CompressionNone {
		var err error
		bodyBytes, err = codec.Compress(bodyBytes, msg.Header.Compression)
		if err != nil {
			return nil, err
		}
	}

	headerCodec, err := codec.New(codec.JSON)
	if err != nil {
		return nil, err
	}

	headerBytes, err := headerCodec.Marshal(msg.Header)
	if err != nil {
		return nil, err
	}

	headerLen := uint32(len(headerBytes))
	bodyLen := uint32(len(bodyBytes))

	// 出站校验
	if err := ValidateLen(headerLen, bodyLen); err != nil {
		return nil, err
	}

	total := 2 + 4 + 4 + headerLen + bodyLen
	buf := make([]byte, total)

	binary.BigEndian.PutUint16(buf[0:2], Magic)

	binary.BigEndian.PutUint32(buf[2:6], headerLen)

	binary.BigEndian.PutUint32(buf[6:10], bodyLen)

	copy(buf[10:], headerBytes)

	copy(buf[10+headerLen:], bodyBytes)

	return buf, nil
}

// DecodeHeaderLen 从字节切片解析 headerLen
func DecodeHeaderLen(data []byte) uint32 {
	return binary.BigEndian.Uint32(data)
}

// DecodeBodyLen 从字节切片解析 bodyLen
func DecodeBodyLen(data []byte) uint32 {
	return binary.BigEndian.Uint32(data)
}

// DecodeBytes 从字节数组解码完整的 Message（用于粘包处理）
func Decode(data []byte) (*Message, error) {

	if len(data) < 10 {
		return nil, fmt.Errorf("data too short")
	}

	// 检查 Magic
	if binary.BigEndian.Uint16(data[0:2]) != Magic {
		return nil, fmt.Errorf("invalid magic number")
	}

	headerLen := binary.BigEndian.Uint32(data[2:6])
	bodyLen := binary.BigEndian.Uint32(data[6:10])

	// 入站校验
	if err := ValidateLen(headerLen, bodyLen); err != nil {
		return nil, err
	}

	totalLen := HeaderFixedLen + int(headerLen) + int(bodyLen)
	if len(data) < totalLen {
		return nil, fmt.Errorf("incomplete packet")
	}

	headerBytes := data[10 : 10+headerLen]

	headerCodec, err := codec.New(codec.JSON)
	if err != nil {
		return nil, err
	}

	var header Header
	if err := headerCodec.Unmarshal(headerBytes, &header); err != nil {
		return nil, err
	}

	// 读取 body
	bodyBytes := data[10+headerLen : 10+headerLen+bodyLen]

	if header.Compression != codec.CompressionNone {
		bodyBytes, err = codec.Decompress(bodyBytes, header.Compression)
		if err != nil {
			return nil, err
		}
	}

	return &Message{
		Header: &header,
		Body:   bodyBytes,
	}, nil
}
