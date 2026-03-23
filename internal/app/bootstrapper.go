package app

import (
	"context"
	"fmt"

	"github.com/gammazero/workerpool"
	"github.com/mrPTqp/total-recaller/internal/bot"
	"github.com/mrPTqp/total-recaller/internal/config"
	"github.com/mrPTqp/total-recaller/internal/service"
	"github.com/mrPTqp/total-recaller/internal/storage"
	"github.com/mrPTqp/total-recaller/internal/storage/postgres"
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
	workerPool := workerpool.New(bs.cfg.Bot.PoolSize)

	db, err := storage.NewDatabase(
		bs.cfg.DatabaseDSN,
		bs.cfg.Database.Pool.MaxOpenConns,
		bs.cfg.Database.Pool.MaxIdleConns,
		bs.cfg.Database.Pool.MaxLifetime,
		bs.cfg.Database.Pool.MaxIdleTime,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}

	userRepo := postgres.NewRetryUserRepository(
		postgres.NewUserRepository(db.GetDB()),
		bs.cfg.Database.Retry.MaxAttempts,
		bs.cfg.Database.Retry.Backoff,
	)
	meetingRepo := postgres.NewRetryMeetingRepository(
		postgres.NewMeetingRepository(db.GetDB()),
		bs.cfg.Database.Retry.MaxAttempts,
		bs.cfg.Database.Retry.Backoff,
	)

	userService := service.NewUserService(userRepo, bs.logger)
	meetingService := service.NewMeetingService(meetingRepo, bs.logger)

	botClient, err := bot.NewClient(bs.cfg, bs.logger, workerPool, meetingService, userService)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot client: %w", err)
	}

	components := &AppComponents{
		Config:   bs.cfg,
		Logger:   bs.logger,
		Bot:      botClient,
		Database: db,
	}

	return components, nil
}

type AppComponents struct {
	Config   *config.Config
	Logger   *zap.Logger
	Bot      *bot.Client
	Database *storage.Database
}
