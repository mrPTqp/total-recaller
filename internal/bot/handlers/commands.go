package handlers

import (
	"fmt"
	"github.com/gammazero/workerpool"
	"github.com/mrPTqp/total-recaller/internal/config"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

type CommandHandlers struct {
	bot        *tele.Bot
	cfg        *config.Config
	logger     *zap.Logger
	workerPool *workerpool.WorkerPool
}

func NewCommandHandlers(bot *tele.Bot, cfg *config.Config, logger *zap.Logger, workerPool *workerpool.WorkerPool) *CommandHandlers {
	return &CommandHandlers{
		bot:        bot,
		cfg:        cfg,
		logger:     logger,
		workerPool: workerPool,
	}
}

func (h *CommandHandlers) HandleStart(ctx tele.Context) error {
	user := ctx.Sender()

	h.logger.Info("User started bot", zap.String("username", user.Username))

	message := fmt.Sprintf(
		"Привет, %s! 👋\n\n"+
			"Я - умный помощник для конспектирования встреч.\n\n"+
			"Вот что я умею:\n"+
			"/list - Показать список встреч\n"+
			"/get <id> - Получить транскрипцию встречи\n"+
			"/find <ключевые слова> - Найти встречу по ключевым словам\n"+
			"/chat - Задать вопрос ИИ-ассистенту\n\n"+
			"Просто отправь мне аудиофайл или голосовое сообщение, и я создам транскрипцию!",
		user.FirstName,
	)

	return ctx.Send(message)
}

func (h *CommandHandlers) HandleList(ctx tele.Context) error {
	message := "Список встреч:\n\n1. Встреча с командой разработки (2024-01-15)\n2. Совещание по проекту (2024-01-10)"
	return ctx.Send(message)
}

func (h *CommandHandlers) HandleGet(ctx tele.Context) error {
	args := ctx.Args()
	if len(args) == 0 {
		return ctx.Send("Пожалуйста, укажите ID встречи. Пример: /get 1")
	}

	meetingID := args[0]
	message := fmt.Sprintf("Транскрипция встречи %s:\n\n[Текст транскрипции]", meetingID)
	return ctx.Send(message)
}

func (h *CommandHandlers) HandleFind(ctx tele.Context) error {
	args := ctx.Args()
	if len(args) == 0 {
		return ctx.Send("Пожалуйста, укажите ключевые слова для поиска. Пример: /find проект")
	}

	keywords := ctx.Text()[5:]
	message := fmt.Sprintf("Результаты поиска по запросу '%s':\n\n1. Встреча с командой (2024-01-15)\n2. Совещание (2024-01-10)", keywords)
	return ctx.Send(message)
}

func (h *CommandHandlers) HandleChat(ctx tele.Context) error {
	args := ctx.Args()
	if len(args) == 0 {
		return ctx.Send("Пожалуйста, задайте вопрос. Пример: /chat Какие были решения по проекту?")
	}

	question := ctx.Text()[5:]
	message := fmt.Sprintf("Анализирую ваш вопрос: '%s'\n\n[Ответ от GigaChat]", question)
	return ctx.Send(message)
}