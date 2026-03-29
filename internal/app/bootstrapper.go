package app

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/gammazero/workerpool"
	"github.com/mrPTqp/total-recaller/internal/bot"
	"github.com/mrPTqp/total-recaller/internal/bot/handlers"
	"github.com/mrPTqp/total-recaller/internal/config"
	"github.com/mrPTqp/total-recaller/internal/llm"
	"github.com/mrPTqp/total-recaller/internal/queue"
	"github.com/mrPTqp/total-recaller/internal/service"
	"github.com/mrPTqp/total-recaller/internal/storage"
	"github.com/mrPTqp/total-recaller/internal/storage/postgres"
	"github.com/mrPTqp/total-recaller/internal/token"
	"github.com/mrPTqp/total-recaller/internal/transcriber"
	"go.uber.org/zap"
	telegramm "gopkg.in/telebot.v3"
)

type Bootstrapper struct {
	cfg    *config.Config
	logger *zap.Logger
}

func NewBootstrapper(cfg *config.Config, logger *zap.Logger) *Bootstrapper {
	return &Bootstrapper{cfg: cfg, logger: logger}
}

func (bs *Bootstrapper) MustRun(ctx context.Context) (*AppComponents, error) {
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

	tokenManagerHttpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	transcriberHttpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	LLMHttpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	tokenManager := token.NewTokenManager(bs.cfg, tokenManagerHttpClient, bs.logger)

	transcriberClient := transcriber.NewTranscriberClient(bs.cfg, transcriberHttpClient, tokenManager, bs.logger)
	gigachatClient := llm.NewLLMClient(bs.cfg, LLMHttpClient, tokenManager, bs.logger)

	userService := service.NewUserService(userRepo, bs.logger)
	meetingService := service.NewMeetingService(meetingRepo, userRepo, bs.logger)

	queueManager := queue.NewQueueManager(bs.cfg)
	workerManager := queue.NewWorkerManager(bs.logger, transcriberClient, gigachatClient)

	settings := telegramm.Settings{
		Token:  bs.cfg.BotToken,
		Poller: &telegramm.LongPoller{Timeout: 10 * time.Second},
	}

	tbot, err := telegramm.NewBot(settings)
	if err != nil {
		bs.logger.Fatal("Failed to create bot", zap.Error(err))
		return nil, err
	}

	workerPool := workerpool.New(bs.cfg.Bot.PoolSize)
	cmdHandlers := handlers.NewCommandHandlers(bs.logger, workerPool, meetingService, userService, queueManager)
	evtHandlers := handlers.NewEventHandlers(bs.cfg, bs.logger, workerPool, meetingService, queueManager)

	botClient, err := bot.NewClient(tbot, bs.cfg, bs.logger, workerPool, cmdHandlers, evtHandlers)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot client: %w", err)
	}

	components := &AppComponents{
		Config:            bs.cfg,
		Logger:            bs.logger,
		Bot:               botClient,
		Database:          db,
		TokenManager:      tokenManager,
		HTTPClient:        transcriberHttpClient,
		TranscriberClient: transcriberClient,
		QueueManager:      queueManager,
		WorkerManager:     workerManager,
	}

	return components, nil
}

type AppComponents struct {
	Config            *config.Config
	Logger            *zap.Logger
	Bot               *bot.Client
	Database          *storage.Database
	TokenManager      *token.TokenManager
	HTTPClient        *http.Client
	TranscriberClient *transcriber.TranscriberClient
	QueueManager      *queue.QueueManager
	WorkerManager     *queue.WorkerManager
}
