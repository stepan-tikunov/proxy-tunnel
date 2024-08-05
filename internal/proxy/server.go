package proxy

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"

	"github.com/stepan-tikunov/proxy-tunnel/internal/config"
	"github.com/stepan-tikunov/proxy-tunnel/internal/payload"

	"github.com/google/uuid"
)

type Server struct {
	clientMu sync.Mutex
	publicMu sync.Mutex

	publicCh, clientCh chan net.Conn
	clientConn         net.Conn
	publicConns        map[uuid.UUID]net.Conn

	cfg config.Server
	log *slog.Logger
}

func listenTCPConnections(address string) (chan net.Conn, error) {
	const op = "proxy.listenTCPConnections"

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	res := make(chan net.Conn)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				close(res)
				return
			}

			res <- conn
		}
	}()

	return res, nil
}

func NewServer(cfg config.Server, log *slog.Logger) *Server {
	return &Server{cfg: cfg, log: log, publicConns: make(map[uuid.UUID]net.Conn)}
}

func (s *Server) Listen(ctx context.Context) error {
	const op = "proxy.New"

	pubCh, err := listenTCPConnections(fmt.Sprintf(":%d", s.cfg.PublicPort))
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	s.publicCh = pubCh

	cltCh, err := listenTCPConnections(fmt.Sprintf(":%d", s.cfg.ClientPort))
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	s.clientCh = cltCh

	s.log.Info("started proxy server",
		slog.Int("publicPort", s.cfg.PublicPort),
		slog.Int("clientPort", s.cfg.ClientPort),
	)

	s.handleConnections(ctx)

	s.log.Info("stopped proxy server")

	return nil
}

func (s *Server) handlePublicConn(ctx context.Context, conn net.Conn) {
	s.publicMu.Lock()

	id := uuid.New()
	s.publicConns[id] = conn

	s.publicMu.Unlock()

	defer func() {
		conn.Close()

		s.publicMu.Lock()
		delete(s.publicConns, id)
		s.publicMu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			buf := make([]byte, payload.MaxPayloadSize)
			n, err := conn.Read(buf)
			if err != nil {
				s.log.Info("connection dropped",
					slog.String("id", id.String()),
					slog.Any("error", err),
				)
				return
			}

			if s.clientConn == nil {
				s.log.Info("dropping connection (client not yet connected)")
				return
			}

			p := payload.New(id, buf[:n])

			if _, err = s.clientConn.Write(p.Bytes()); err != nil {
				s.log.Error("could not send data to client", slog.Any("error", err))
				return
			}
		}
	}
}

func (s *Server) handleClientConn(ctx context.Context, conn net.Conn) {
	s.clientMu.Lock()
	defer func() {
		conn.Close()
		s.clientMu.Unlock()
	}()

	s.clientConn = conn

	for {
		select {
		case <-ctx.Done():
			return
		default:
			p, err := payload.Read(conn)
			if err != nil {
				s.log.Error("could not read response data", slog.Any("error", err))
				continue
			}

			pubConn, ok := s.publicConns[p.ID]
			if !ok {
				s.log.Error("could not forward response data, public client's socket not found")
				continue
			}

			s.publicMu.Lock()

			_, err = pubConn.Write(p.Data)
			if err != nil {
				s.log.Error("could not forward response data", slog.Any("error", err))
			}

			s.publicMu.Unlock()
		}
	}
}

func (s *Server) handleConnections(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case conn := <-s.publicCh:
			s.log.Info("received public connection",
				slog.String("remote", conn.RemoteAddr().String()),
			)
			go s.handlePublicConn(ctx, conn)
		case conn := <-s.clientCh:
			s.log.Info("received client connection",
				slog.String("remote", conn.RemoteAddr().String()),
			)
			go s.handleClientConn(ctx, conn)
		}
	}
}
