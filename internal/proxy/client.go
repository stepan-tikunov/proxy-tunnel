package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/stepan-tikunov/proxy-tunnel/internal/config"
	"github.com/stepan-tikunov/proxy-tunnel/internal/payload"

	"github.com/google/uuid"
)

type Client struct {
	cfg config.Client
	log *slog.Logger

	mu    sync.Mutex
	conns map[uuid.UUID]net.Conn
}

func NewClient(cfg config.Client, log *slog.Logger) *Client {
	return &Client{
		cfg:   cfg,
		log:   log,
		conns: make(map[uuid.UUID]net.Conn),
	}
}

func (c *Client) Connect(ctx context.Context) error {
	const op = "proxy.Client.Connect"

	conn, err := net.Dial("tcp", c.cfg.ServerAddr)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	c.log.Info("started proxy client",
		slog.Int("port", c.cfg.Port),
		slog.String("serverAddr", c.cfg.ServerAddr),
	)

	requests := c.requestsChan(ctx, conn)
	for req := range requests {
		if err = c.forwardRequest(ctx, req, conn); err != nil {
			c.log.Error("could not forward request", slog.Any("error", err))
		}
	}

	c.log.Info("stopped proxy client")

	return nil
}

func (c *Client) forwardRequest(ctx context.Context, req payload.Payload, respConn net.Conn) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if conn, ok := c.conns[req.ID]; ok {
		_, err := conn.Write(req.Data)
		return err
	}

	conn, err := net.Dial("tcp", fmt.Sprintf(":%d", c.cfg.Port))
	if err != nil {
		return err
	}

	c.conns[req.ID] = conn

	c.log.Debug("sending request data",
		slog.String("id", req.ID.String()),
		slog.Int("length", len(req.Data)),
		slog.String("data", string(req.Data)),
	)

	_, err = conn.Write(req.Data)
	_ = conn.SetDeadline(time.Now().Add(c.cfg.Timeout))
	if err != nil {
		return err
	}
	c.log.Debug("sent request data",
		slog.String("id", req.ID.String()),
		slog.Int("length", len(req.Data)),
	)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				buf := make([]byte, payload.MaxDataSize)
				c.log.Debug("reading response data",
					slog.String("id", req.ID.String()),
				)
				n, err := conn.Read(buf)

				if err != nil {
					if payload.IsTimeout(err) {
						n = 0
						conn.Close()
					} else {
						c.log.Error("could not read response data", slog.Any("error", err))
						return
					}
				}

				p := payload.New(req.ID, buf[:n])
				c.log.Debug("finished reading response data",
					slog.String("id", req.ID.String()),
					slog.Int("length", len(p.Data)),
					slog.String("data", string(p.Data)),
				)
				if _, err := respConn.Write(p.Bytes()); err != nil {
					c.log.Error("could not send response data", slog.Any("error", err))
					return
				}
			}
		}
	}()

	return nil
}

func (c *Client) requestsChan(ctx context.Context, conn net.Conn) chan payload.Payload {
	res := make(chan payload.Payload, 100)

	go func() {
		defer close(res)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				_ = conn.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
				p, err := payload.Read(conn)

				if err != nil {
					if payload.IsTimeout(err) {
						continue
					}

					if errors.Is(err, io.EOF) {
						c.log.Error("lost connection to server, stopping", slog.Any("error", err))
						return
					}

					c.log.Error("could not read request data", slog.Any("error", err))
					return
				}

				c.log.Debug("received request data",
					slog.String("id", p.ID.String()),
					slog.Int("length", len(p.Data)),
					slog.String("data", string(p.Data)),
				)

				res <- *p
			}
		}
	}()

	return res
}
