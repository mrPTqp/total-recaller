package app

import (
	"context"

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
}

type AppComponents struct {
	Config          *config.Config
	Logger          *zap.Logger
}
