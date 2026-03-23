package app

import (
	"context"

	"github.com/mrPTqp/total-recaller/internal/bot"
	"github.com/mrPTqp/total-recaller/internal/storage"
	"go.uber.org/zap"
)

type App struct {
	c      *AppComponents
	bot    *bot.Client
	database *storage.Database
}

func NewApp(components *AppComponents) *App {
	return &App{
		c:        components,
		bot:      components.Bot,
		database: components.Database,
	}
}

func (a *App) RunWithContext(ctx context.Context) {
	if a.bot != nil {
		go a.bot.Start(ctx)
	}
}

func (a *App) Shutdown(ctx context.Context) {
	if a.bot != nil {
		a.bot.Stop()
	}
	
	if a.database != nil {
		if err := a.database.Close(); err != nil {
			a.c.Logger.Error("Warning: failed to close database connection", zap.Error(err))
		}
	}
}
