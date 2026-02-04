package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Versifine/locus/internal/config"
	"github.com/Versifine/locus/internal/logger"
	"github.com/Versifine/locus/internal/proxy"
)

func main() {
	logger.Init(logger.Config{
		Level:     "info",
		Format:    "text",
		AddSource: true,
	})

	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	server := proxy.NewServer(
		fmt.Sprintf("%s:%d", cfg.Listen.Host, cfg.Listen.Port),
		fmt.Sprintf("%s:%d", cfg.Backend.Host, cfg.Backend.Port),
	)
	err = server.Start(ctx)
	if err != nil {
		slog.Error("failed to start server", "error", err)
		os.Exit(1)
	}

}
