package main

import (
	"go.uber.org/zap"

	"github.com/mrPTqp/total-recaller/internal/config"
	"github.com/mrPTqp/total-recaller/pkg/logger"
)

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
		zap.Any("config: ", cfg),
	)

	log = logger.NewLogger()
	defer func() {
		_ = log.Sync()
	}()

	log.Info("Total Recaller application is running")
}
