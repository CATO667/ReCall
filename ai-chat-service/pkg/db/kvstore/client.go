package kvstore

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

type Client struct {
	addr string
	conn net.Conn
	r    *bufio.Reader
}

func NewClient(host string, port int) (*Client, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return nil, err
	}
	c := &Client{
		addr: addr,
		conn: conn,
		r:    bufio.NewReader(conn),
	}
	if err := c.ping(); err != nil {
		conn.Close()
		return nil, err
	}
	return c, nil
}

func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Client) ping() error {
	reply, err := c.Command(context.Background(), "PING")
	if err != nil {
		return err
	}
	if reply.Type != '+' || reply.Status != "PONG" {
		return fmt.Errorf("unexpected PING reply: %+v", reply)
	}
	return nil
}

func (c *Client) Command(ctx context.Context, args ...string) (*Reply, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	deadline, ok := ctx.Deadline()
	if ok {
		_ = c.conn.SetDeadline(deadline)
	} else {
		_ = c.conn.SetDeadline(time.Now().Add(10 * time.Second))
	}
	if _, err := c.conn.Write(encodeCommand(args...)); err != nil {
		return nil, err
	}
	return readReply(c.r)
}

func (c *Client) Get(ctx context.Context, key string) ([]byte, error) {
	reply, err := c.Command(ctx, "GET", key)
	if err != nil {
		return nil, err
	}
	if reply.IsNil {
		return nil, nil
	}
	if reply.Type == '$' {
		return reply.Bulk, nil
	}
	if reply.Type == '-' {
		return nil, fmt.Errorf("%s", reply.Error)
	}
	return nil, fmt.Errorf("unexpected GET reply: %+v", reply)
}

func (c *Client) Set(ctx context.Context, key, value string) error {
	reply, err := c.Command(ctx, "SET", key, value)
	if err != nil {
		return err
	}
	if reply.Type == '+' && reply.Status == "OK" {
		return nil
	}
	// mykvstore 键已存在时返回 +key exist（非 -key exist）
	if reply.Type == '+' && reply.Status == "key exist" {
		return ErrKeyExists
	}
	if reply.Type == '-' && strings.Contains(reply.Error, "exist") {
		return ErrKeyExists
	}
	if reply.Type == '-' {
		return fmt.Errorf("%s", reply.Error)
	}
	return fmt.Errorf("unexpected SET reply: %+v", reply)
}

func (c *Client) Mod(ctx context.Context, key, value string) error {
	reply, err := c.Command(ctx, "MOD", key, value)
	if err != nil {
		return err
	}
	if reply.Type == '+' && reply.Status == "OK" {
		return nil
	}
	if reply.Type == '-' {
		return fmt.Errorf("%s", reply.Error)
	}
	return fmt.Errorf("unexpected MOD reply: %+v", reply)
}

func (c *Client) Del(ctx context.Context, key string) error {
	reply, err := c.Command(ctx, "DEL", key)
	if err != nil {
		return err
	}
	if reply.Type == '+' {
		return nil
	}
	if reply.Type == '-' {
		return fmt.Errorf("%s", reply.Error)
	}
	return nil
}

func (c *Client) Expire(ctx context.Context, key string, ttlSec int) error {
	reply, err := c.Command(ctx, "EXPIRE", key, fmt.Sprintf("%d", ttlSec))
	if err != nil {
		return err
	}
	if reply.Type == '+' {
		return nil
	}
	if reply.Type == '-' {
		return fmt.Errorf("%s", reply.Error)
	}
	return nil
}

func (c *Client) SetOrMod(ctx context.Context, key, value string) error {
	err := c.Set(ctx, key, value)
	if err == ErrKeyExists {
		return c.Mod(ctx, key, value)
	}
	return err
}

func (c *Client) SetWithExpire(ctx context.Context, key, value string, ttlSec int) error {
	if err := c.SetOrMod(ctx, key, value); err != nil {
		return err
	}
	return c.Expire(ctx, key, ttlSec)
}

var ErrKeyExists = fmt.Errorf("key exist")
