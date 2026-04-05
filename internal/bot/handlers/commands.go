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
	"github.com/mrPTqp/total-recaller/internal/queue"
	"github.com/mrPTqp/total-recaller/internal/service"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

type CommandHandlers struct {
	logger         *zap.Logger
	wp             *workerpool.WorkerPool
	meetingService *service.MeetingService
	userService    *service.UserService
	queueManager   *queue.QueueManager
}

func NewCommandHandlers(
	logger *zap.Logger,
	wp *workerpool.WorkerPool,
	meetingService *service.MeetingService,
	userService *service.UserService,
	queueManager *queue.QueueManager,
) *CommandHandlers {
	return &CommandHandlers{
		logger:         logger,
		wp:             wp,
		meetingService: meetingService,
		userService:    userService,
		queueManager:   queueManager,
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
			"/sfind <запрос по смыслу> - Найти встречу по смыслу\n"+
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

		var message strings.Builder
		message.WriteString("Список встреч:\n\n")

		count := 0
		for meeting := range h.meetingService.ListMeetings(ctxWithTimeout, ctx.Sender().ID) {
			count++
			fmt.Fprintf(&message, "%d. %s (ID: %d)\n", count,
				meeting.CreatedAt.Format("2006-01-02 15:04"), meeting.ID)
		}

		if count == 0 {
			ctx.Send("У вас пока нет сохраненных встреч")
			return
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

		var message strings.Builder
		message.WriteString("Результаты поиска:\n\n")

		count := 0
		for meeting := range h.meetingService.SearchMeetings(ctxWithTimeout, ctx.Sender().ID, query) {
			count++
			fmt.Fprintf(&message, "%d. %s (ID: %d)\n", count,
				meeting.CreatedAt.Format("2006-01-02 15:04"), meeting.ID)
		}

		if count == 0 {
			ctx.Send("По вашему запросу ничего не найдено")
			return
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

	taskID := queue.GenerateMessageID()
	task := queue.LLMTask{
		ID:        taskID,
		UserID:    ctx.Sender().ID,
		TaskType:  queue.LLMTaskTypeChat,
		Text:      "", // Not used for chat tasks
		Query:     question,
		CreatedAt: time.Now(),
	}

	select {
	case h.queueManager.LLMTasks <- task:
		h.logger.Info("Sent chat task to LLM queue",
			zap.String("task_id", taskID),
			zap.String("question", question))
	default:
		h.logger.Info("LLM channel is full, cannot send chat task")
		return ctx.Send("Извините, система перегружена. Пожалуйста, попробуйте позже.")
	}

	go func() {
		ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		select {
		case result := <-h.queueManager.LLMResults:
			if result.TaskID == taskID {
				if result.Error != nil {
					h.logger.Error("Failed to get response from LLM", zap.Error(result.Error))
					ctx.Send("Извините, произошла ошибка при обработке вашего запроса. Пожалуйста, попробуйте позже.")
					return
				}

				h.logger.Info("Successfully received response from LLM", zap.String("response", result.Response))
				ctx.Send(result.Response)
			}
		case <-ctxWithTimeout.Done():
			h.logger.Info("Timeout while waiting for LLM response")
			ctx.Send("Извините, запрос обрабатывается слишком долго. Пожалуйста, попробуйте позже.")
		}
	}()

	return nil
}

func (h *CommandHandlers) HandleSemanticFind(ctx tele.Context) error {
	args := ctx.Args()
	if len(args) == 0 {
		return ctx.Send("Пожалуйста, укажите поисковый запрос. Пример: /sfind Как проходило собрание по проекту?")
	}

	query := strings.Join(args, " ")

	h.wp.Submit(func() {
		ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		embeddingTaskID := queue.GenerateMessageID()
		embeddingTask := queue.EmbeddingTask{
			ID:        embeddingTaskID,
			UserID:    ctx.Sender().ID,
			Text:      query,
			FileID:    "", // Not used for search queries
			CreatedAt: time.Now(),
		}

		select {
		case h.queueManager.EmbeddingTasks <- embeddingTask:
			h.logger.Info("Sent semantic search task to embedding queue",
				zap.String("task_id", embeddingTaskID),
				zap.String("query", query))
		default:
			h.logger.Info("Embedding channel is full, cannot send semantic search task")
			ctx.Send("Извините, система перегружена. Пожалуйста, попробуйте позже.")
			return
		}

		var queryEmbedding []float32
		select {
		case result := <-h.queueManager.EmbeddingResults:
			if result.TaskID == embeddingTaskID {
				if result.Error != nil {
					h.logger.Error("Failed to generate query embedding", zap.Error(result.Error))
					ctx.Send("Извините, произошла ошибка при обработке запроса. Пожалуйста, попробуйте позже.")
					return
				}
				queryEmbedding = result.Embedding
			}
		case <-ctxWithTimeout.Done():
			h.logger.Info("Timeout while waiting for query embedding")
			ctx.Send("Извините, запрос обрабатывается слишком долго. Пожалуйста, попробуйте позже.")
			return
		}

		// Search meetings by embedding using iterator
		var message strings.Builder
		message.WriteString("Результаты семантического поиска:\n\n")

		count := 0
		for meeting := range h.meetingService.SearchMeetingsByEmbedding(ctxWithTimeout, ctx.Sender().ID, queryEmbedding, 100, 0) {
			count++
			fmt.Fprintf(&message, "%d. %s (ID: %d)\n", count,
				meeting.CreatedAt.Format("2006-01-02 15:04"), meeting.ID)
		}

		if count == 0 {
			ctx.Send("По вашему запросу ничего не найдено")
			return
		}

		ctx.Send(message.String())
	})

	return nil
}
