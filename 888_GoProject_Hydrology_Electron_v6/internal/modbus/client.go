package modbus

import (
	"context"
	"fmt"
	"sync"
	"time"

	gm "github.com/goburrow/modbus"
)

type Client struct {
	address string
	timeout time.Duration

	mu      sync.Mutex
	handler *gm.TCPClientHandler
	client  gm.Client
}

func NewClient(address string, timeout time.Duration) *Client {
	return &Client{address: address, timeout: timeout}
}

func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.handler != nil {
		_ = c.handler.Close()
	}

	h := gm.NewTCPClientHandler(c.address)
	h.Timeout = c.timeout

	if err := h.Connect(); err != nil {
		c.handler = nil
		c.client = nil
		return fmt.Errorf("connect %s: %w", c.address, err)
	}

	c.handler = h
	c.client = gm.NewClient(h)
	return nil
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.handler != nil {
		_ = c.handler.Close()
	}
	c.handler = nil
	c.client = nil
}

func (c *Client) With(fn func(gm.Client, *gm.TCPClientHandler) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client == nil || c.handler == nil {
		return fmt.Errorf("modbus client is not connected")
	}
	return fn(c.client, c.handler)
}

// 新增带context版本

func (c *Client) WithCtx(ctx context.Context, fn func(gm.Client, *gm.TCPClientHandler) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client == nil || c.handler == nil {
		return fmt.Errorf("modbus client is not connected")
	}

	ch := make(chan error, 1)
	go func() {
		ch <- fn(c.client, c.handler)
	}()

	select {
	case <-ctx.Done():
		// ctx取消，关闭连接打断阻塞IO
		_ = c.handler.Close()
		return ctx.Err()
	case err := <-ch:
		return err
	}
}
