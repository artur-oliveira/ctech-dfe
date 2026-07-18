package main

import (
	"log/slog"
	"os"

	"gopkg.aoctech.app/dfe/api/internal/app"

	"go.uber.org/fx"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	fx.New(app.Module).Run()
}
