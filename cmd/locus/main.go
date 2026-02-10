package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Versifine/locus/internal/agent"
	"github.com/Versifine/locus/internal/bot"
	"github.com/Versifine/locus/internal/config"
	"github.com/Versifine/locus/internal/llm"
	"github.com/Versifine/locus/internal/logger"
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
	switch cfg.Mode {
	case "proxy":
		// startProxyServer(ctx, cfg)
		slog.Info("暂时关闭server功能")
	case "bot":
		startBot(ctx, cfg)
	default:
		slog.Error("Invalid mode in config", "mode", cfg.Mode)
		os.Exit(1)
	}
}
func startBot(ctx context.Context, cfg *config.Config) {
	bot := bot.NewBot(
		fmt.Sprintf("%s:%d", cfg.Backend.Host, cfg.Backend.Port),
		cfg.Bot.Username,
	)
	llmClient := llm.NewLLMClient(&cfg.LLM)
	_ = agent.NewAgent(bot.Bus(), bot, bot, llmClient)
	if err := bot.Start(ctx); err != nil {
		slog.Error("Bot encountered an error", "error", err)
		os.Exit(1)
	}
}

// func startProxyServer(ctx context.Context, cfg *config.Config) {
// 	server := proxy.NewServer(
// 		fmt.Sprintf("%s:%d", cfg.Listen.Host, cfg.Listen.Port),
// 		fmt.Sprintf("%s:%d", cfg.Backend.Host, cfg.Backend.Port),
// 	)
// 	llmClient := llm.NewLLMClient(&cfg.LLM)
// 	_ = agent.NewAgent(server.Bus(), server, server, llmClient)
// 	err := server.Start(ctx)
// 	if err != nil {
// 		slog.Error("Failed to start server", "error", err)
// 		os.Exit(1)
// 	}
// }
