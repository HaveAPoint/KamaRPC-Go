package client

import (
	"errors"
	"kamaRPC/internal/codec"
	"kamaRPC/internal/loadbalance"
	"time"
)

type ClientOption func(*Client) error

func WithClientCodec(t codec.Type) ClientOption {
	return func(c *Client) error {
		cc, err := codec.New(t)
		if err != nil {
			return err
		}
		c.codec = cc
		return nil
	}
}

func WithClientTimeout(d time.Duration) ClientOption {
	return func(c *Client) error {
		c.timeout = d
		return nil
	}
}

func WithClientLoadBalancer(lb loadbalance.LoadBalancer) ClientOption {
	return func(c *Client) error {
		c.lb = lb
		return nil
	}
}

// WithPoolSize 设置每个地址的最大连接数。
// 单条连接已支持 requestID 多路复用，所以默认 1 条就够；
// 调大主要是为了绕开单连接的写锁串行和内核缓冲区瓶颈。
func WithPoolSize(n int) ClientOption {
	return func(c *Client) error {
		if n < 1 {
			return errors.New("pool size must be at least 1")
		}
		c.poolSize = n
		return nil
	}
}
