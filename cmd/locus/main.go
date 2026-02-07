package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Versifine/locus/internal/agent"
	"github.com/Versifine/locus/internal/config"
	"github.com/Versifine/locus/internal/logger"
	"github.com/Versifine/locus/internal/proxy"
)

func main() {

	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}
	logger.Init(logger.Config{
		Level:  cfg.Logging.Level,
		Format: "console",
	})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	server := proxy.NewServer(
		fmt.Sprintf("%s:%d", cfg.Listen.Host, cfg.Listen.Port),
		fmt.Sprintf("%s:%d", cfg.Backend.Host, cfg.Backend.Port),
	)
	agent.NewAgent(server.Bus(), server)
	err = server.Start(ctx)
	if err != nil {
		slog.Error("Failed to start server", "error", err)
		os.Exit(1)
	}

}
