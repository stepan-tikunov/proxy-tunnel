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
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	cfg := config.MustLoad[config.Client]()

	ctx, cancel := context.WithCancel(context.Background())

	client := proxy.NewClient(cfg, log)

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

		<-stop

		cancel()
	}()

	if err := client.Connect(ctx); err != nil {
		panic(err)
	}
}
