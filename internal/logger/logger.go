package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"
)

type Config struct {
	Level  string
	Format string // "text", "json", "console"
	Output io.Writer
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
		level := parseLevel(cfg.Level)
		var handler slog.Handler
		switch cfg.Format {
		case "json":
			handler = slog.NewJSONHandler(cfg.Output, &slog.HandlerOptions{Level: level})
		case "text":
			handler = slog.NewTextHandler(cfg.Output, &slog.HandlerOptions{Level: level})
		default:
			handler = &consoleHandler{w: cfg.Output, level: level}
		}
		lg = slog.New(handler)
		slog.SetDefault(lg)
	})
}

func L() *slog.Logger {
	if lg == nil {
		Init(Config{Level: "debug", Format: "console"})
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

// consoleHandler outputs human-friendly log lines:
//
//	12:00:00 INFO  Starting proxy server  listener=:25565 backend=127.0.0.1:25565
type consoleHandler struct {
	w     io.Writer
	level slog.Level
	attrs []slog.Attr
	group string
}

func (h *consoleHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *consoleHandler) Handle(_ context.Context, r slog.Record) error {
	ts := r.Time.Format(time.TimeOnly) // "15:04:05"
	lvl := levelTag(r.Level)

	line := fmt.Sprintf("%s %s %s", ts, lvl, r.Message)

	// pre-attached attrs (from WithAttrs)
	for _, a := range h.attrs {
		line += formatAttr(h.group, a)
	}
	// per-record attrs
	r.Attrs(func(a slog.Attr) bool {
		line += formatAttr(h.group, a)
		return true
	})

	line += "\n"
	_, err := fmt.Fprint(h.w, line)
	return err
}

func (h *consoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &consoleHandler{
		w:     h.w,
		level: h.level,
		attrs: append(append([]slog.Attr{}, h.attrs...), attrs...),
		group: h.group,
	}
}

func (h *consoleHandler) WithGroup(name string) slog.Handler {
	prefix := name
	if h.group != "" {
		prefix = h.group + "." + name
	}
	return &consoleHandler{
		w:     h.w,
		level: h.level,
		attrs: append([]slog.Attr{}, h.attrs...),
		group: prefix,
	}
}

func levelTag(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return "ERROR"
	case l >= slog.LevelWarn:
		return "WARN "
	case l >= slog.LevelInfo:
		return "INFO "
	default:
		return "DEBUG"
	}
}

func formatAttr(group string, a slog.Attr) string {
	key := a.Key
	if group != "" {
		key = group + "." + key
	}
	return fmt.Sprintf("  %s=%v", key, a.Value)
}
