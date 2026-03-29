package handlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/gammazero/workerpool"
	"github.com/mrPTqp/total-recaller/internal/config"
	"github.com/mrPTqp/total-recaller/internal/models"
	"github.com/mrPTqp/total-recaller/internal/queue"
	"github.com/mrPTqp/total-recaller/internal/service"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

type EventHandlers struct {
	cfg               *config.Config
	logger            *zap.Logger
	wp                *workerpool.WorkerPool
	meetingService    *service.MeetingService
	queueManager      *queue.QueueManager
}

func NewEventHandlers(
	cfg *config.Config,
	logger *zap.Logger,
	workerPool *workerpool.WorkerPool,
	meetingService *service.MeetingService,
	queueManager *queue.QueueManager,
) *EventHandlers {
	return &EventHandlers{
		cfg:               cfg,
		logger:            logger,
		wp:                workerPool,
		meetingService:    meetingService,
		queueManager:      queueManager,
	}
}

func (h *EventHandlers) HandleVoice(ctx tele.Context) error {
	user := ctx.Sender()
	voice := ctx.Message().Voice

	h.logger.Info("Received voice message",
		zap.String("username", user.Username),
		zap.Int64("file_size", voice.FileSize),
	)

	if err := h.validateFileSize(ctx, user.Username, voice.FileSize); err != nil {
		return err
	}

	processingMsg := "⏳ Обрабатываю аудио..."
	sentMsg, err := ctx.Bot().Send(ctx.Chat(), processingMsg)
	if err != nil {
		return err
	}

	taskID := queue.GenerateMessageID()
	task := queue.TranscriberTask{
		ID:          taskID,
		UserID:      user.ID,
		AudioData:   nil, // Will be set after downloading
		AudioFormat: "OPUS",
		FileID:      voice.FileID,
		CreatedAt:   time.Now(),
	}

	h.wp.Submit(func() {
		h.processAudioTask(ctx, user, task, sentMsg)
	})

	return nil
}

func (h *EventHandlers) HandleAudio(ctx tele.Context) error {
	user := ctx.Sender()
	audio := ctx.Message().Audio

	h.logger.Info("Received audio file",
		zap.String("username", user.Username),
		zap.String("title", audio.Title),
		zap.Int64("file_size", audio.FileSize),
	)

	if err := h.validateFileSize(ctx, user.Username, audio.FileSize); err != nil {
		return err
	}

	extension := strings.ToLower(filepath.Ext(audio.FileName))
	if extension != ".mp3" {
		h.logger.Warn("Unsupported audio format",
			zap.String("username", user.Username),
			zap.String("extension", extension),
			zap.String("filename", audio.FileName))

		errorMsg := "❌ Поддерживается только формат MP3. Пожалуйста, загрузите аудиофайл в формате .mp3"
		_, err := ctx.Bot().Send(ctx.Chat(), errorMsg)
		if err != nil {
			h.logger.Error("Failed to send unsupported format error message", zap.Error(err))
		}
		return nil
	}

	processingMsg := "⏳ Обрабатываю аудио..."
	sentMsg, err := ctx.Bot().Send(ctx.Chat(), processingMsg)
	if err != nil {
		return err
	}

	taskID := queue.GenerateMessageID()
	task := queue.TranscriberTask{
		ID:          taskID,
		UserID:      user.ID,
		AudioData:   nil, // Will be set after downloading
		AudioFormat: "MP3",
		FileID:      audio.FileID,
		CreatedAt:   time.Now(),
	}

	h.wp.Submit(func() {
		h.processAudioTask(ctx, user, task, sentMsg)
	})

	return nil
}


func (h *EventHandlers) processAudioTask(ctx tele.Context, user *tele.User, task queue.TranscriberTask, sentMsg *tele.Message) {
	wpCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Download audio file
	rc, err := h.downloadAudioFile(ctx, task.FileID)
	if err != nil {
		h.logger.Error("Failed to download audio file", zap.Error(err))
		return
	}
	defer rc.Close()

	// Update task with audio data
	task.AudioData = rc

	// Send task to transcriber channel
	select {
	case h.queueManager.TranscriberTasks <- task:
		h.logger.Info("Sent task to transcriber queue",
			zap.String("task_id", task.ID),
			zap.String("file_id", task.FileID))
	default:
		h.logger.Info("Transcriber channel is full, cannot send audio task")
		return
	}

	// Wait for transcription result
	transcriberResult, err := h.waitForTranscriptionResult(wpCtx, task.ID)
	if err != nil {
		h.logger.Error("Failed to get transcription result", zap.Error(err))
		return
	}

	// Handle transcription errors or empty results
	if transcriberResult.Error != nil {
		h.handleTranscriptionError(ctx, sentMsg, transcriberResult.Error)
		return
	}

	if transcriberResult.Transcription == "" {
		h.handleEmptyTranscription(ctx, sentMsg)
		return
	}

	// Create meeting record
	meeting, err := h.createMeetingRecord(wpCtx, user.ID, transcriberResult.FileID, transcriberResult.Transcription)
	if err != nil {
		h.logger.Error("Failed to create meeting", zap.Error(err))
		return
	}
	h.logger.Info("Meeting record created successfully", zap.Int("meeting_id", meeting.ID))

	// Send embedding generation task to queue
	embeddingTaskID := queue.GenerateMessageID()
	embeddingTask := queue.EmbeddingTask{
		ID:        embeddingTaskID,
		UserID:    user.ID,
		Text:      transcriberResult.Transcription,
		FileID:    transcriberResult.FileID,
		CreatedAt: time.Now(),
	}

	select {
	case h.queueManager.EmbeddingTasks <- embeddingTask:
		h.logger.Info("Sent embedding task to queue",
			zap.String("task_id", embeddingTaskID))
	default:
		h.logger.Info("Embedding channel is full, cannot send embedding task")
		return
	}

	// Send summarization task to LLM
	summaryTaskID := queue.GenerateMessageID()
	summaryTask := queue.LLMTask{
		ID:        summaryTaskID,
		UserID:    user.ID,
		TaskType:  queue.LLMTaskTypeSummarize,
		Text:      transcriberResult.Transcription,
		Query:     "", // Not used for summarization
		CreatedAt: time.Now(),
	}

	select {
	case h.queueManager.LLMTasks <- summaryTask:
		h.logger.Info("Sent summarization task to LLM queue",
			zap.String("task_id", summaryTaskID))
	default:
		h.logger.Info("LLM channel is full, cannot send summarization task")
		return
	}

	// Wait for summarization result
	summaryResult, err := h.waitForSummarizationResult(wpCtx, summaryTaskID)
	if err != nil {
		h.logger.Error("Failed to get summarization result", zap.Error(err))
		return
	}

	// Handle summarization errors
	if summaryResult.Error != nil {
		h.handleSummarizationError(ctx, sentMsg, summaryResult.Error)
		return
	}

	// Update meeting with summary
	err = h.meetingService.UpdateMeetingSummary(wpCtx, user.ID, transcriberResult.FileID, summaryResult.Response)
	if err != nil {
		h.logger.Error("Failed to update meeting with summary", zap.Error(err))
		return
	}
	h.logger.Info("Meeting updated with summary successfully")

	// Wait for embedding result
	embeddingResult, err := h.waitForEmbeddingResult(wpCtx, embeddingTaskID)
	if err != nil {
		h.logger.Error("Failed to get embedding result", zap.Error(err))
		return
	}

	// Handle embedding errors
	if embeddingResult.Error != nil {
		h.logger.Error("Failed to generate embedding", zap.Error(embeddingResult.Error))
		return
	}

	// Update meeting with embedding
	err = h.meetingService.UpdateMeetingEmbedding(wpCtx, user.ID, transcriberResult.FileID, embeddingResult.Embedding)
	if err != nil {
		h.logger.Error("Failed to update meeting with embedding", zap.Error(err))
		return
	}
	h.logger.Info("Meeting updated with embedding successfully")

	// Edit message with summary
	if _, err := ctx.Bot().Edit(sentMsg, summaryResult.Response); err != nil {
		h.logger.Error("Failed to edit message", zap.Error(err))
		return
	}
	h.logger.Info("Message edited successfully")
}

func (h *EventHandlers) downloadAudioFile(ctx tele.Context, fileID string) (io.ReadCloser, error) {
	h.logger.Info("Getting audio file info by ID", zap.String("file_id", fileID))
	f, err := ctx.Bot().FileByID(fileID)
	if err != nil {
		h.logger.Error("Failed to get audio file info by ID", zap.Error(err))
		return nil, err
	}

	h.logger.Info("Downloading audio file")
	rc, err := ctx.Bot().File(&f)
	if err != nil {
		h.logger.Error("Failed download audio file", zap.Error(err))
		return nil, err
	}

	return rc, nil
}

func (h *EventHandlers) waitForTranscriptionResult(ctx context.Context, taskID string) (*queue.TranscriberResult, error) {
	select {
	case result := <-h.queueManager.TranscriberResults:
		if result.TaskID == taskID {
			return &result, nil
		}
	case <-ctx.Done():
		h.logger.Info("Timeout while waiting for transcription result")
		return nil, ctx.Err()
	}
	return nil, errors.New("transcription result not found")
}

func (h *EventHandlers) waitForSummarizationResult(ctx context.Context, taskID string) (*queue.LLMResult, error) {
	select {
	case result := <-h.queueManager.LLMResults:
		if result.TaskID == taskID {
			return &result, nil
		}
	case <-ctx.Done():
		h.logger.Info("Timeout while waiting for summarization result")
		return nil, ctx.Err()
	}
	return nil, errors.New("summarization result not found")
}

func (h *EventHandlers) waitForEmbeddingResult(ctx context.Context, taskID string) (*queue.EmbeddingResult, error) {
	select {
	case result := <-h.queueManager.EmbeddingResults:
		if result.TaskID == taskID {
			return &result, nil
		}
	case <-ctx.Done():
		h.logger.Info("Timeout while waiting for embedding result")
		return nil, ctx.Err()
	}
	return nil, errors.New("embedding result not found")
}

func (h *EventHandlers) createMeetingRecord(ctx context.Context, userID int64, fileID, transcription string) (*models.Meeting, error) {
	h.logger.Info("Creating meeting record")
	meeting, err := h.meetingService.CreateMeeting(ctx, userID, fileID, transcription)
	if err != nil {
		h.logger.Error("Failed to create meeting", zap.Error(err))
		return nil, err
	}
	return meeting, nil
}

func (h *EventHandlers) handleTranscriptionError(ctx tele.Context, sentMsg *tele.Message, err error) {
	h.logger.Error("Failed to transcribe audio", zap.Error(err))
	errorMsg := "❌ Не удалось распознать аудио. Пожалуйста, попробуйте еще раз."
	if _, editErr := ctx.Bot().Edit(sentMsg, errorMsg); editErr != nil {
		h.logger.Error("Failed to send error message", zap.Error(editErr))
	}
}

func (h *EventHandlers) handleEmptyTranscription(ctx tele.Context, sentMsg *tele.Message) {
	h.logger.Warn("Transcription returned empty result")
	emptyMsg := "⚠️ Аудио распознано, но текст не найден. Возможно, аудио слишком короткое или содержит только шум."
	if _, editErr := ctx.Bot().Edit(sentMsg, emptyMsg); editErr != nil {
		h.logger.Error("Failed to send empty result message", zap.Error(editErr))
	}
}

func (h *EventHandlers) handleSummarizationError(ctx tele.Context, sentMsg *tele.Message, err error) {
	h.logger.Error("Failed to summarize with Gigachat", zap.Error(err))
	errorMsg := "❌ Не удалось создать краткое содержание. Пожалуйста, попробуйте еще раз."
	if _, editErr := ctx.Bot().Edit(sentMsg, errorMsg); editErr != nil {
		h.logger.Error("Failed to send error message", zap.Error(editErr))
	}
}

func (h *EventHandlers) validateFileSize(ctx tele.Context, username string, fileSize int64) error {
	if fileSize > h.cfg.Bot.MaxFileSize {
		h.logger.Warn("Audio exceeds file size limit",
			zap.String("username", username),
			zap.Int64("file_size", fileSize),
			zap.Int64("max_file_size", h.cfg.Bot.MaxFileSize))

		errorMsg := fmt.Sprintf("❌ Размер превышает лимит в %.1f МБ. Пожалуйста, отправьте более короткое audio.", float64(h.cfg.Bot.MaxFileSize)/1024/1024)
		_, err := ctx.Bot().Send(ctx.Chat(), errorMsg)
		if err != nil {
			h.logger.Error("Failed to send file size limit error message", zap.Error(err))
		}
		return errors.New("file size exceeds limit")
	}
	return nil
}

