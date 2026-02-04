package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/Versifine/locus/internal/config"
	"github.com/Versifine/locus/internal/logger"
	"github.com/Versifine/locus/internal/proxy"
)

func main() {
	logger.Init(logger.Config{
		Level:     "debug",
		Format:    "text",
		AddSource: true,
	})

	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	server := proxy.NewServer(
		fmt.Sprintf("%s:%d", cfg.Listen.Host, cfg.Listen.Port),
		fmt.Sprintf("%s:%d", cfg.Backend.Host, cfg.Backend.Port),
	)
	err = server.Start()
	if err != nil {
		slog.Error("failed to start server", "error", err)
		os.Exit(1)
	}

}
