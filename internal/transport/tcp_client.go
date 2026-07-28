package transport

import (
	"errors"
	"kamaRPC/internal/protocol"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var ErrConnectionClosed = errors.New("connection closed")

type TCPClient struct {
	conn *TCPConnection
	addr string

	writeMu sync.Mutex
	seq     uint64

	pending sync.Map // map[uint64]*Future

	closed int32
}

func newTCPClient(addr string) (*TCPClient, error) {
	rawConn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, err
	}

	c := &TCPClient{
		conn: NewTCPConnection(rawConn),
		addr: addr,
	}

	go c.readLoop()
	return c, nil
}

func (c *TCPClient) nextSeq() uint64 {
	return atomic.AddUint64(&c.seq, 1)
}

func (c *TCPClient) completePending(seq uint64, res []byte, err error) bool {
	value, loaded := c.pending.LoadAndDelete(seq)
	if !loaded {
		return false
	}

	value.(*Future).Done(res, err)
	return true
}

func (c *TCPClient) SendAsync(msg *protocol.Message) (*Future, error) {
	if atomic.LoadInt32(&c.closed) == 1 {
		return nil, ErrConnectionClosed
	}

	seq := c.nextSeq()
	msg.Header.RequestID = seq

	future := NewFuture()
	future.setCancel(func(err error) {
		c.completePending(seq, nil, err)
	})
	c.pending.Store(seq, future)

	c.writeMu.Lock()
	err := c.conn.Write(msg)
	c.writeMu.Unlock()

	if err != nil {
		c.pending.Delete(seq)
		c.fail(err) // 关键：write 失败也要彻底杀死连接(解决之前连接bug)
		return nil, err
	}

	return future, nil
}

func (c *TCPClient) readLoop() {
	for {
		msg, err := c.conn.Read()
		if err != nil {
			c.fail(err)
			return
		}

		seq := msg.Header.RequestID

		if msg.Header.Error != "" {
			c.completePending(seq, nil, errors.New(msg.Header.Error))
		} else {
			c.completePending(seq, msg.Body, nil)
		}
	}
}

func (c *TCPClient) closeWithError(cause error) error {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return nil
	}

	closeErr := c.conn.Close()

	c.pending.Range(func(key, _ any) bool {
		value, loaded := c.pending.LoadAndDelete(key)
		if !loaded {
			return true
		}

		future := value.(*Future)
		future.Done(nil, cause)
		return true
	})

	return closeErr
}

func (c *TCPClient) fail(err error) {
	_ = c.closeWithError(err)
}

func (c *TCPClient) Close() error {
	return c.closeWithError(ErrConnectionClosed)
}
