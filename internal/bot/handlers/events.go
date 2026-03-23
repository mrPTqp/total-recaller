package handlers

import (
	"github.com/gammazero/workerpool"
	"github.com/mrPTqp/total-recaller/internal/config"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

type EventHandlers struct {
	bot    *tele.Bot
	cfg    *config.Config
	logger *zap.Logger
	wp     *workerpool.WorkerPool
}

func NewEventHandlers(bot *tele.Bot, cfg *config.Config, logger *zap.Logger, workerPool *workerpool.WorkerPool) *EventHandlers {
	return &EventHandlers{
		bot:    bot,
		cfg:    cfg,
		logger: logger,
		wp:     workerPool,
	}
}

func (h *EventHandlers) HandleVoice(ctx tele.Context) error {
	user := ctx.Sender()
	voice := ctx.Message().Voice

	h.logger.Info("Received voice message",
		zap.String("username", user.Username),
		zap.Int64("file_size", voice.FileSize),
	)

	processingMsg := "⏳ Обрабатываю аудиозапись..."
	sentMsg, err := ctx.Bot().Send(ctx.Chat(), processingMsg)
	if err != nil {
		return err
	}

	task := func() {
		h.logger.Info("Processing voice message", zap.String("username", user.Username))

		resultMsg := "✅ Аудиозапись обработана! Транскрипция сохранена."
		if _, err := ctx.Bot().Edit(sentMsg, resultMsg); err != nil {
			h.logger.Error("Failed to edit message", zap.Error(err))
		}
	}

	h.wp.Submit(task)

	return nil
}

func (h *EventHandlers) HandleAudio(ctx tele.Context) error {
	user := ctx.Sender()
	audio := ctx.Message().Audio

	h.logger.Info("Received audio file",
		zap.String("username", user.Username),
		zap.String("title", audio.Title),
	)

	processingMsg := "⏳ Обрабатываю аудиофайл..."
	sentMsg, err := ctx.Bot().Send(ctx.Chat(), processingMsg)
	if err != nil {
		return err
	}

	task := func() {
		h.logger.Info("Processing audio file", zap.String("username", user.Username))

		resultMsg := "✅ Аудиофайл обработан! Транскрипция сохранена."
		if _, err := ctx.Bot().Edit(sentMsg, resultMsg); err != nil {
			h.logger.Error("Failed to edit message", zap.Error(err))
		}
	}

	h.wp.Submit(task)

	return nil
}
