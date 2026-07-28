package protocol

import (
	"errors"
	"testing"
)

// TestEncode_RejectsOversizedBody 出站也要拦，避免发出对端必然拒绝的包。
// 注意 Header 不设 Compression：gzip 会把全零 body 压到几 KB，
// 长度上限管的是线上字节数，不是原始数据大小。
func TestEncode_RejectsOversizedBody(t *testing.T) {
	msg := &Message{
		Header: &Header{RequestID: 1},
		Body:   make([]byte, MaxBodySize+1),
	}

	if _, err := Encode(msg); !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("Encode() error = %v, want %v", err, ErrBodyTooLarge)
	}
}

// TestEncode_AcceptsBodyAtLimit 边界值：正好等于上限应通过
func TestEncode_AcceptsBodyAtLimit(t *testing.T) {
	msg := &Message{
		Header: &Header{RequestID: 1},
		Body:   make([]byte, MaxBodySize),
	}

	if _, err := Encode(msg); err != nil {
		t.Fatalf("Encode() error = %v, want nil", err)
	}
}

func TestValidateLen(t *testing.T) {
	tests := []struct {
		name      string
		headerLen uint32
		bodyLen   uint32
		wantErr   error
	}{
		{"都在范围内", 100, 1024, nil},
		{"header 正好等于上限", MaxHeaderSize, 0, nil},
		{"body 正好等于上限", 100, MaxBodySize, nil},
		{"header 超一字节", MaxHeaderSize + 1, 0, ErrHeaderTooLarge},
		{"body 超一字节", 100, MaxBodySize + 1, ErrBodyTooLarge},
		{"uint32 最大值不会溢出成负数", 0xFFFFFFFF, 0xFFFFFFFF, ErrHeaderTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateLen(tt.headerLen, tt.bodyLen); !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidateLen(%d, %d) = %v, want %v",
					tt.headerLen, tt.bodyLen, err, tt.wantErr)
			}
		})
	}
}
