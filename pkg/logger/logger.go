package logger

import (
	"log/slog"
	"os"
)

var Log *slog.Logger

func init() {
	Init()
}

func Init() {
	handler := slog.NewJSONHandler(
		os.Stdout,
		&slog.HandlerOptions{
			Level: slog.LevelInfo,
			ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
				switch attr.Key {
				case slog.LevelKey:
					attr.Key = "severity"
				case slog.MessageKey:
					attr.Key = "message"
				}
				return attr
			},
		},
	)

	Log = slog.New(handler)
	slog.SetDefault(Log)
}
