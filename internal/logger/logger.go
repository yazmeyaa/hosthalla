package logger

import (
	"io"
	"log/slog"
)

type LoggerParams struct {
	Output io.Writer
	Level  slog.Level
}

func NewLogger(params LoggerParams) *slog.Logger {
	logger := slog.New(slog.NewTextHandler(params.Output, &slog.HandlerOptions{Level: params.Level}))
	logger = logger.With(
		slog.String("service", "hosthalla"),
	)
	return logger
}
