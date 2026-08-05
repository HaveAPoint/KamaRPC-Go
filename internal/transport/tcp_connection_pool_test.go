package transport

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
)

// newFakeClient 造一条不走真实网络的 TCPClient，用于只关心 closed 标志的池子测试。
func newFakeClient(t *testing.T) *TCPClient {
	t.Helper()

	local, peer := net.Pipe()
	t.Cleanup(func() {
		local.Close()
		peer.Close()
	})

	return &TCPClient{conn: NewTCPConnection(local)}
}

// 池满且首条连接已死时，Acquire 必须复用后面那条活连接，而不是重新建连。
// 旧实现边遍历边删，下标递增与元素左移叠加会跳过 conns[1]，从而误判“全死了”。
func TestPool_AcquireReusesLiveConnAfterDeadOne(t *testing.T) {
	dead := newFakeClient(t)
	atomic.StoreInt32(&dead.closed, 1)
	live := newFakeClient(t)

	p := NewConnectionPool("127.0.0.1:0", 2)
	p.conns = []*TCPClient{dead, live}

	conn, err := p.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}

	if conn != live {
		t.Fatalf("Acquire() returned %p, want the live conn %p", conn, live)
	}

	// 死连接必须已被摘掉，且不会因为 maxActive 未满就再补一条
	if len(p.conns) != 1 || p.conns[0] != live {
		t.Fatalf("conns = %v, want exactly [live]", p.conns)
	}
}

// 多条活连接时，连续 Acquire 应该按游标轮转，而不是每次都返回同一条。
func TestPool_AcquireRoundRobins(t *testing.T) {
	a, b := newFakeClient(t), newFakeClient(t)

	p := NewConnectionPool("127.0.0.1:0", 2)
	p.conns = []*TCPClient{a, b}

	got := make([]*TCPClient, 4)
	for i := range got {
		conn, err := p.Acquire(context.Background())
		if err != nil {
			t.Fatalf("Acquire() #%d error = %v", i, err)
		}
		got[i] = conn
	}

	want := []*TCPClient{a, b, a, b}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Acquire() #%d = %p, want %p", i, got[i], want[i])
		}
	}
}

// 全部连接都死掉时，池子应该清空后重建，而不是返回一条死连接。
func TestPool_AcquireRebuildsWhenAllDead(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { conn.Close() })
		}
	}()

	dead := newFakeClient(t)
	atomic.StoreInt32(&dead.closed, 1)

	p := NewConnectionPool(listener.Addr().String(), 1)
	p.conns = []*TCPClient{dead}

	conn, err := p.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	defer conn.Close()

	if conn == dead {
		t.Fatal("Acquire() returned the dead conn, want a freshly dialed one")
	}
	if atomic.LoadInt32(&conn.closed) != 0 {
		t.Fatal("Acquire() returned a closed conn")
	}
}
