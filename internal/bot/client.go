package bot

import (
	"context"
	"time"

	"github.com/gammazero/workerpool"
	"github.com/mrPTqp/total-recaller/internal/bot/handlers"
	"github.com/mrPTqp/total-recaller/internal/config"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

type Client struct {
	bot         *tele.Bot
	cfg         *config.Config
	logger      *zap.Logger
	workerPool  *workerpool.WorkerPool
	cmdHandlers *handlers.CommandHandlers
	evtHandlers *handlers.EventHandlers
}

func NewClient(cfg *config.Config, logger *zap.Logger) (*Client, error) {
	settings := tele.Settings{
		Token:  cfg.BotToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := tele.NewBot(settings)
	if err != nil {
		logger.Fatal("Failed to create bot", zap.Error(err))
		return nil, err
	}

	workerPool := workerpool.New(cfg.Bot.PoolSize)

	cmdHandlers := handlers.NewCommandHandlers(bot, cfg, logger, workerPool)
	evtHandlers := handlers.NewEventHandlers(bot, cfg, logger, workerPool)

	client := &Client{
		bot:         bot,
		cfg:         cfg,
		logger:      logger,
		workerPool:  workerPool,
		cmdHandlers: cmdHandlers,
		evtHandlers: evtHandlers,
	}

	client.registerCommandRoutes()
	client.registerEventRoutes()

	return client, nil
}

func (c *Client) Start(ctx context.Context) {
	c.logger.Info("Starting bot", zap.Int("workers", c.cfg.Bot.PoolSize))
	c.bot.Start()
	c.logger.Info("Bot started successfully")
}

func (c *Client) Stop() {
	c.workerPool.StopWait()
	c.bot.Stop()
}

func (c *Client) registerCommandRoutes() {
	c.bot.Handle("/start", c.cmdHandlers.HandleStart)
	c.bot.Handle("/list", c.cmdHandlers.HandleList)
	c.bot.Handle("/get", c.cmdHandlers.HandleGet)
	c.bot.Handle("/find", c.cmdHandlers.HandleFind)
	c.bot.Handle("/chat", c.cmdHandlers.HandleChat)

	commands := []tele.Command{
		{Text: "start", Description: "Зарегистрироваться"},
		{Text: "list", Description: "Получить список сохраненных встреч"},
		{Text: "get", Description: "Получить текст конкретной встречи"},
		{Text: "find", Description: "Найти встречу по ключевым словам"},
		{Text: "chat", Description: "Отправить запрос к GigaChat"},
	}

	c.bot.SetCommands(commands)
}

func (c *Client) registerEventRoutes() {
	c.bot.Handle(tele.OnVoice, c.evtHandlers.HandleVoice)
	c.bot.Handle(tele.OnAudio, c.evtHandlers.HandleAudio)
}