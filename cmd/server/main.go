package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/stepan-tikunov/proxy-tunnel/internal/config"
	"github.com/stepan-tikunov/proxy-tunnel/internal/proxy"
)

func main() {
	cfg := config.MustLoad[config.Server]()
	log := slog.New(slog.NewJSONHandler(
		os.Stdout,
		&slog.HandlerOptions{Level: cfg.Env.LogLevel()},
	))

	ctx, cancel := context.WithCancel(context.Background())

	server := proxy.NewServer(cfg, log)

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

		<-stop

		cancel()
	}()

	if err := server.Listen(ctx); err != nil {
		panic(err)
	}
}
