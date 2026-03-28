package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mrPTqp/total-recaller/internal/bot"
	"github.com/mrPTqp/total-recaller/internal/queue"
	"github.com/mrPTqp/total-recaller/internal/storage"
	"github.com/mrPTqp/total-recaller/internal/token"
	"go.uber.org/zap"
)

type App struct {
	c            *AppComponents
	bot          *bot.Client
	database     *storage.Database
	ticker       *time.Ticker
	tokenManager *token.TokenManager
	queueManager *queue.QueueManager
	workerManager *queue.WorkerManager
	shutdown     sync.Once
}

func NewApp(components *AppComponents) *App {
	return &App{
		c:            components,
		bot:          components.Bot,
		database:     components.Database,
		tokenManager: components.TokenManager,
		queueManager: components.QueueManager,
		workerManager: components.WorkerManager,
		ticker:       time.NewTicker(components.Config.Transcriber.TokenManager.RefreshInterval),
	}
}

func (a *App) RunWithContext(ctx context.Context) {
	// Start queue workers
	a.queueManager.StartWorkers(ctx, a.workerManager.TranscriberWorker, a.workerManager.LLMWorker)
	
	// Start bot
	if a.bot != nil {
		go a.bot.Start(ctx)
	}

	if err := a.tokenManager.RefreshToken(ctx, a.c.Config.Transcriber.TokenManager.Scope); err != nil {
		a.c.Logger.Error("Failed to get initial token", zap.Error(err))
	}
	if err := a.tokenManager.RefreshToken(ctx, a.c.Config.LLM.TokenManager.Scope); err != nil {
		a.c.Logger.Error("Failed to get initial token", zap.Error(err))
	}

	a.runBackgroundJobs(ctx)

	a.c.Logger.Info("application started")
}

func (a *App) runBackgroundJobs(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("background job ticker stopped %v", ctx.Err())
			return
		case <-a.ticker.C:
			backupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			if err := a.tokenManager.RefreshToken(backupCtx, a.c.Config.Transcriber.TokenManager.Scope); err != nil {
				fmt.Printf("failed to refresh token %v", err)
			}
			cancel()
		}
	}
}

func (a *App) Shutdown(ctx context.Context) {
	a.shutdown.Do(func() {
		if a.bot != nil {
			a.bot.Stop()
		}

		a.ticker.Stop()

		// Close queue channels and wait for workers to finish
		a.queueManager.Close()
		a.queueManager.Wait()

		if a.database != nil {
			if err := a.database.Close(); err != nil {
				a.c.Logger.Error("Warning: failed to close database connection", zap.Error(err))
			}
		}
	})
}
