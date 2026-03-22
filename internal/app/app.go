package app

import (
	"context"

	"github.com/mrPTqp/total-recaller/internal/bot"
)

type App struct {
	c    *AppComponents
	bot  *bot.Client
}

func NewApp(components *AppComponents) *App {
	return &App{
		c:   components,
		bot: components.Bot,
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
}
