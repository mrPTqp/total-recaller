package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gammazero/workerpool"
	"github.com/mrPTqp/total-recaller/internal/config"
	"github.com/mrPTqp/total-recaller/internal/llm"
	"github.com/mrPTqp/total-recaller/internal/service"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

type CommandHandlers struct {
	bot            *tele.Bot
	cfg            *config.Config
	logger         *zap.Logger
	wp             *workerpool.WorkerPool
	meetingService *service.MeetingService
	userService    *service.UserService
	llmClient      *llm.LLMClient
}

func NewCommandHandlers(
	bot *tele.Bot,
	cfg *config.Config,
	logger *zap.Logger,
	wp *workerpool.WorkerPool,
	meetingService *service.MeetingService,
	userService *service.UserService,
	llmClient *llm.LLMClient,
) *CommandHandlers {
	return &CommandHandlers{
		bot:            bot,
		cfg:            cfg,
		logger:         logger,
		wp:             wp,
		meetingService: meetingService,
		userService:    userService,
		llmClient:      llmClient,
	}
}

func (h *CommandHandlers) HandleStart(ctx tele.Context) error {
	user := ctx.Sender()

	h.wp.Submit(func() {
		ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := h.userService.RegisterUser(ctxWithTimeout, user.ID, user.Username, user.FirstName, user.LastName)
		if err != nil {
			h.logger.Error("Failed to register user", zap.Int64("telegram_id", user.ID), zap.Error(err))
			return
		}

		h.logger.Info("User started bot", zap.String("username", user.Username))
	})

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
	h.wp.Submit(func() {
		ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		meetings, err := h.meetingService.ListMeetings(ctxWithTimeout, ctx.Sender().ID)
		if err != nil {
			h.logger.Error("Failed to list meetings", zap.Error(err))
			ctx.Send("Ошибка при получении списка встреч")
			return
		}

		if len(meetings) == 0 {
			ctx.Send("У вас пока нет сохраненных встреч")
			return
		}

		var message strings.Builder
		message.WriteString("Список встреч:\n\n")
		for i, meeting := range meetings {
			fmt.Fprintf(&message, "%d. %s (ID: %d)\n", i+1,
				meeting.CreatedAt.Format("2006-01-02 15:04"), meeting.ID)
		}

		ctx.Send(message.String())
	})

	return nil
}

func (h *CommandHandlers) HandleGet(ctx tele.Context) error {
	args := ctx.Args()
	if len(args) == 0 {
		return ctx.Send("Пожалуйста, укажите ID встречи. Пример: /get 1")
	}

	meetingID, err := strconv.Atoi(args[0])
	if err != nil {
		return ctx.Send("Некорректный ID встречи")
	}

	h.wp.Submit(func() {
		ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		meeting, err := h.meetingService.GetMeeting(ctxWithTimeout, meetingID, ctx.Sender().ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				ctx.Send("Встреча не найдена")
				return
			}
			h.logger.Error("Failed to get meeting", zap.Int("id", meetingID), zap.Error(err))
			ctx.Send("Ошибка при получении транскрипции")
			return
		}

		if meeting.FullText == "" {
			ctx.Send("Транскрипция для этой встречи еще не готова")
			return
		}

		message := fmt.Sprintf("Транскрипция встречи %d:\n\n%s", meetingID, meeting.FullText)
		ctx.Send(message)
	})

	return nil
}

func (h *CommandHandlers) HandleFind(ctx tele.Context) error {
	args := ctx.Args()
	if len(args) == 0 {
		return ctx.Send("Пожалуйста, укажите ключевые слова для поиска. Пример: /find интеграция с гигачат")
	}

	query := strings.Join(args, " ")

	h.wp.Submit(func() {
		ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		meetings, err := h.meetingService.SearchMeetings(ctxWithTimeout, ctx.Sender().ID, query, 100, 0)
		if err != nil {
			h.logger.Error("Failed to search meetings", zap.String("query", query), zap.Error(err))
			ctx.Send("Ошибка при поиске встреч")
			return
		}

		if len(meetings) == 0 {
			ctx.Send("По вашему запросу ничего не найдено")
			return
		}

		var message strings.Builder
		message.WriteString("Результаты поиска:\n\n")
		for i, meeting := range meetings {
			fmt.Fprintf(&message, "%d. %s (ID: %d)\n", i+1,
				meeting.CreatedAt.Format("2006-01-02 15:04"), meeting.ID)
		}

		ctx.Send(message.String())
	})

	return nil
}

func (h *CommandHandlers) HandleChat(ctx tele.Context) error {
	args := ctx.Args()
	if len(args) == 0 {
		return ctx.Send("Пожалуйста, задайте вопрос. Пример: /chat Какие были решения по проекту?")
	}

	question := ctx.Text()[5:]

	h.wp.Submit(func() {
		ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		h.logger.Info("Processing chat request", zap.String("question", question))
		
		response, err := h.llmClient.Chat(ctxWithTimeout, question)
		if err != nil {
			h.logger.Error("Failed to get response from LLM", zap.Error(err))
			ctx.Send("Извините, произошла ошибка при обработке вашего запроса. Пожалуйста, попробуйте позже.")
			return
		}

		h.logger.Info("Successfully received response from LLM", zap.String("response", response))
		ctx.Send(response)
	})

	return nil
}
