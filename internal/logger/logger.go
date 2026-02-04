package logger

import (
	"io"
	"log/slog"
	"os"
	"sync"
)

type Config struct {
	Level     string
	Format    string // "text" or "json"
	AddSource bool
	Output    io.Writer
}

var (
	once sync.Once
	lg   *slog.Logger
)

func Init(cfg Config) {
	once.Do(func() {
		if cfg.Output == nil {
			cfg.Output = os.Stdout
		}
		opts := &slog.HandlerOptions{
			Level:     parseLevel(cfg.Level),
			AddSource: cfg.AddSource,
		}
		var handler slog.Handler
		switch cfg.Format {
		case "text":
			handler = slog.NewTextHandler(cfg.Output, opts)
		case "json":
			handler = slog.NewJSONHandler(cfg.Output, opts)
		default:
			handler = slog.NewTextHandler(cfg.Output, opts)
		}
		lg = slog.New(handler)
		slog.SetDefault(lg)
	})
}
func L() *slog.Logger {
	if lg == nil {
		Init(Config{
			Level:     "debug",
			Format:    "text",
			AddSource: true,
		})
	}
	return lg
}

func parseLevel(levelStr string) slog.Level {
	switch levelStr {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
