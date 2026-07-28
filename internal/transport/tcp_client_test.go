package transport

//模拟一个正在等待响应的 Future，然后关闭 TCPClient，验证 Future 是否会收到连接关闭错误。
import (
	"errors"
	"net"
	"testing"
	"time"
)

func TestTCPClient_CloseCompletesPendingFutures(t *testing.T) {
	clientConn, peerConn := net.Pipe()
	defer peerConn.Close()

	client := &TCPClient{
		conn: NewTCPConnection(clientConn),
	}

	future := NewFuture()
	client.pending.Store(uint64(1), future)

	go client.readLoop()

	if err := client.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	select {
	case <-future.DoneChan():
		_, err := future.Wait()
		if !errors.Is(err, ErrConnectionClosed) {
			t.Fatalf("Wait() error = %v, want %v", err, ErrConnectionClosed)
		}

	case <-time.After(200 * time.Millisecond):
		t.Fatal("pending Future remained blocked after TCPClient.Close()")
	}
}
