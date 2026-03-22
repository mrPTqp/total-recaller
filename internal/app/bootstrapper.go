package app

import (
	"context"
	"fmt"

	"github.com/mrPTqp/total-recaller/internal/bot"
	"github.com/mrPTqp/total-recaller/internal/config"
	"go.uber.org/zap"
)

type Bootstrapper struct {
	cfg    *config.Config
	logger *zap.Logger
}

func NewBootstrapper(cfg *config.Config, logger *zap.Logger) *Bootstrapper {
	return &Bootstrapper{cfg: cfg, logger: logger}
}

func (bs *Bootstrapper) MustRun(ctx context.Context) (*AppComponents, error) {
	botClient, err := bot.NewClient(bs.cfg, bs.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot client: %w", err)
	}

	components := &AppComponents{
		Config: bs.cfg,
		Logger: bs.logger,
		Bot:    botClient,
	}

	return components, nil
}

type AppComponents struct {
	Config          *config.Config
	Logger          *zap.Logger
	Bot             *bot.Client
}
