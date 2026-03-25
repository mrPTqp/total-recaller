package app

import (
	"context"
	"sync"
	"time"
	"fmt"

	"github.com/mrPTqp/total-recaller/internal/bot"
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
	shutdown     sync.Once
}

func NewApp(components *AppComponents) *App {
	return &App{
		c:            components,
		bot:          components.Bot,
		database:     components.Database,
		tokenManager: components.TokenManager,
		ticker:       time.NewTicker(components.Config.Transcriber.TokenManager.RefreshInterval),
	}
}

func (a *App) RunWithContext(ctx context.Context) {
	if a.bot != nil {
		go a.bot.Start(ctx)
	}

	// Initialize token manager with initial token
	if err := a.tokenManager.RefreshToken(ctx); err != nil {
		a.c.Logger.Error("Failed to get initial token", zap.Error(err))
	}

	a.runBackgroundJobs(ctx)
}

func (a *App) runBackgroundJobs(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("background job ticker stopped %v", ctx.Err())
			return
		case <-a.ticker.C:
			backupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			if err := a.tokenManager.RefreshToken(backupCtx); err != nil {
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

		if a.database != nil {
			if err := a.database.Close(); err != nil {
				a.c.Logger.Error("Warning: failed to close database connection", zap.Error(err))
			}
		}
	})
}
