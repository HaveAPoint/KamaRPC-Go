package server

import (
	"kamaRPC/internal/transport"
	"net"
	"sync"
	"testing"
)

// TestServer_ConnTrackingRace 回归：conns 曾经无锁并发读写，
// 可触发 fatal error: concurrent map writes。需配合 -race 运行。
func TestServer_ConnTrackingRace(t *testing.T) {
	s, err := NewServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	const n = 64
	var wg sync.WaitGroup

	// 模拟 Accept 循环登记连接、连接 goroutine 退出时注销
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			local, remote := net.Pipe()
			defer remote.Close()

			conn := transport.NewTCPConnection(local)
			if s.trackConn(conn) {
				s.untrackConn(conn)
			}
			conn.Close()
		}()
	}

	// 同时并发 Shutdown，模拟第三方遍历 conns
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.Shutdown()
	}()

	wg.Wait()

	s.mu.Lock()
	remaining := len(s.conns)
	s.mu.Unlock()

	if remaining != 0 {
		t.Fatalf("len(conns) = %d, want 0", remaining)
	}
}
