package transport

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

var ErrPoolClosed = errors.New("connection pool closed")

type ConnectionPool struct {
	addr string

	maxActive int

	conns []*TCPClient
	mu    sync.Mutex

	closed bool
	next   int
}

func NewConnectionPool(addr string, maxActive int) *ConnectionPool {
	if maxActive < 1 {
		maxActive = 1
	}
	return &ConnectionPool{
		addr:      addr,
		maxActive: maxActive,
		conns:     make([]*TCPClient, 0, maxActive),
	}
}

// Acquire 先清掉已关闭的连接，再决定新建还是复用。
// 清理和轮询必须分成两步：边遍历边删会让后面的元素左移，
// 与递增的下标叠加后会跳过尚未检查的连接。
func (p *ConnectionPool) Acquire(ctx context.Context) (*TCPClient, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil, ErrPoolClosed
	}

	p.removeDeadLocked()

	// 没满就补一条新连接
	if len(p.conns) < p.maxActive {
		conn, err := newTCPClient(p.addr)
		if err == nil {
			p.conns = append(p.conns, conn)
			return conn, nil
		}

		// 建连失败时，池里还有活连接就退化成复用；
		// 否则才把错误抛给调用方
		if len(p.conns) == 0 {
			return nil, err
		}
	}

	// 已满：从上次的位置往后取，把请求摊到各条连接上。
	// 此处剩下的都是刚校验过的活连接；返回后才失效属于固有竞态，
	// 由调用方拿到 ErrConnectionClosed 后重新 Acquire 处理。
	idx := p.next % len(p.conns)
	p.next = (idx + 1) % len(p.conns)
	return p.conns[idx], nil
}

// removeDeadLocked 原地摘掉已被标记关闭的连接。调用方必须持有 p.mu。
func (p *ConnectionPool) removeDeadLocked() {
	alive := p.conns[:0]
	for _, conn := range p.conns {
		if atomic.LoadInt32(&conn.closed) == 0 {
			alive = append(alive, conn)
		}
	}

	// 清掉尾部的残留引用，否则死连接对象不会被回收
	for i := len(alive); i < len(p.conns); i++ {
		p.conns[i] = nil
	}
	p.conns = alive

	// 连接被摘掉后游标可能越界
	if len(p.conns) == 0 {
		p.next = 0
	} else {
		p.next %= len(p.conns)
	}
}

func (p *ConnectionPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}
	p.closed = true

	for _, conn := range p.conns {
		conn.Close()
	}
}
