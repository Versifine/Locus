package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Versifine/locus/internal/agent"
	"github.com/Versifine/locus/internal/body"
	"github.com/Versifine/locus/internal/bot"
	"github.com/Versifine/locus/internal/config"
	debugconsole "github.com/Versifine/locus/internal/debug"
	"github.com/Versifine/locus/internal/llm"
	"github.com/Versifine/locus/internal/logger"
	"github.com/Versifine/locus/internal/skill"
	"github.com/Versifine/locus/internal/skill/behaviors"
	"github.com/Versifine/locus/internal/world"
)

const physicsTickInterval = 50 * time.Millisecond

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
		slog.Info("Proxy mode is disabled for now")
	case "bot":
		if err := startBot(ctx, cfg); err != nil {
			slog.Error("Bot encountered an error", "error", err)
		}
	default:
		slog.Error("Invalid mode in config", "mode", cfg.Mode)
		os.Exit(1)
	}
}

func startBot(ctx context.Context, cfg *config.Config) error {
	b := bot.NewBot(
		fmt.Sprintf("%s:%d", cfg.Backend.Host, cfg.Backend.Port),
		cfg.Bot.Username,
	)
	llmClient := llm.NewLLMClient(&cfg.LLM)

	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()

	botErrCh := make(chan error, 1)
	go func() {
		err := b.Start(runCtx)
		botErrCh <- err
		cancelRun()
	}()

	go func() {
		if err := b.WaitForInitialPosition(runCtx); err != nil {
			if runCtx.Err() == nil {
				slog.Warn("Body loop is not ready because initial position is unavailable", "error", err)
			}
			return
		}

		snap := b.GetState()
		bodyController := body.New(snap.Position, false, b, b, b)
		bodyController.SetEntityProvider(b)
		b.SetLocalPositionSink(bodyController)

		if cfg.Debug {
			console := debugconsole.NewConsole(bodyController, b, b)
			slog.Info("Debug console enabled")
			if err := console.Start(runCtx); err != nil && runCtx.Err() == nil {
				slog.Error("Debug console stopped with error, fallback to idle body loop", "error", err)
				runIdleBodyLoop(runCtx, bodyController, b)
			}
			return
		}

		if cfg.AgentLegacyChat {
			_ = agent.NewAgent(b.Bus(), b, b, llmClient)
			slog.Info("Legacy chat mode enabled")
			runIdleBodyLoop(runCtx, bodyController, b)
			return
		}

		runner := skill.NewBehaviorRunner(b.SendMsgToServer, b.GetState, b)
		idle := behaviors.IdleSpec()
		if ok := runner.Start(idle.Name, idle.Fn, idle.Channels, idle.Priority); !ok {
			slog.Warn("Failed to start idle behavior")
		}

		loopAgent := agent.NewLoopAgent(
			b.Bus(),
			b,
			b,
			bodyController,
			runner,
			llmClient,
			b,
			agent.DefaultCamera(),
		)

		slog.Info("Agent loop enabled")
		if err := loopAgent.Start(runCtx); err != nil && runCtx.Err() == nil {
			slog.Error("Agent loop stopped with error", "error", err)
		}
	}()

	err := <-botErrCh
	if err != nil && ctx.Err() == nil && runCtx.Err() == nil {
		return err
	}
	return nil
}

func runIdleBodyLoop(ctx context.Context, b *body.Body, stateProvider interface{ GetState() world.Snapshot }) {
	ticker := time.NewTicker(physicsTickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snap := stateProvider.GetState()
			input := body.InputState{
				Yaw:   snap.Position.Yaw,
				Pitch: snap.Position.Pitch,
			}
			if err := b.Tick(input); err != nil {
				if strings.Contains(err.Error(), "connection is not initialized") {
					continue
				}
				slog.Warn("Idle body tick failed", "error", err)
			}
		}
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
