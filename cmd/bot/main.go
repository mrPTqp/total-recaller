package main

import (
	"context"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/mrPTqp/total-recaller/internal/config"
	"github.com/mrPTqp/total-recaller/internal/app"
	"github.com/mrPTqp/total-recaller/pkg/logger"
)

//TODO new go features
//TODO add logging
//TODO add tests
//TODO add semantic search
//TODO classify errors

func main() {
	log, err := zap.NewProduction()
	if err != nil {
		panic("failed to create init logger")
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Failed to load configuration: %v", zap.Error(err))
	}

	log.Info("Configuration successfully loaded",
		zap.String("config", cfg.String()),
	)

	log = logger.NewLogger()
	defer func() {
		_ = log.Sync()
	}()

	bootstrapper := app.NewBootstrapper(cfg, log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	components, err := bootstrapper.MustRun(ctx)
	if err != nil {
		log.Fatal("failed to bootstrap application", zap.Error(err))
	}

	application := app.NewApp(components)

	go application.RunWithContext(ctx)
	log.Info("application started")

	<-ctx.Done()
	log.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	application.Shutdown(shutdownCtx)
	log.Info("application stopped")
}
