package handlers

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/gammazero/workerpool"
	"github.com/mrPTqp/total-recaller/internal/config"
	"github.com/mrPTqp/total-recaller/internal/llm"
	"github.com/mrPTqp/total-recaller/internal/service"
	"github.com/mrPTqp/total-recaller/internal/transcriber"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

type EventHandlers struct {
	bot               *tele.Bot
	cfg               *config.Config
	logger            *zap.Logger
	wp                *workerpool.WorkerPool
	meetingService    *service.MeetingService
	transcriberClient *transcriber.TranscriberClient
	gigachatClient    *llm.LLMClient
}

func NewEventHandlers(bot *tele.Bot, cfg *config.Config, logger *zap.Logger, workerPool *workerpool.WorkerPool, meetingService *service.MeetingService, transcriberClient *transcriber.TranscriberClient, gigachatClient *llm.LLMClient) *EventHandlers {
	return &EventHandlers{
		bot:               bot,
		cfg:               cfg,
		logger:            logger,
		wp:                workerPool,
		meetingService:    meetingService,
		transcriberClient: transcriberClient,
		gigachatClient:    gigachatClient,
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

	h.wp.Submit(func() {
		h.logger.Info("Starting voice message processing in worker pool")

		wpCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		fileId := voice.FileID
		h.logger.Info("Getting audio file info by ID", zap.String("file_id", fileId))
		f, err := ctx.Bot().FileByID(fileId)
		if err != nil {
			h.logger.Error("Failed to get audio file info by ID", zap.Error(err))
			return
		}

		h.logger.Info("Downloading audio file")
		rc, err := ctx.Bot().File(&f)
		if err != nil {
			h.logger.Error("Failed download audio file", zap.Error(err))
			return
		}

		defer func() {
			h.logger.Info("Closing audio file reader")
			_ = rc.Close()
		}()

		h.logger.Info("Starting transcription")
		transcription, err := h.transcriberClient.TranscribeFile(wpCtx, rc, "OPUS")
		if err != nil {
			h.logger.Error("Failed to transcribe audio", zap.Error(err))
			// Send error message to user
			errorMsg := "❌ Не удалось распознать аудиозапись. Пожалуйста, попробуйте еще раз."
			if _, editErr := ctx.Bot().Edit(sentMsg, errorMsg); editErr != nil {
				h.logger.Error("Failed to send error message", zap.Error(editErr))
			}
			return
		}
		h.logger.Info("Transcription completed", zap.String("transcription", transcription))

		if transcription == "" {
			h.logger.Warn("Transcription returned empty result")
			emptyMsg := "⚠️ Аудиозапись распознана, но текст не найден. Возможно, запись слишком короткая или содержит только шум."
			if _, editErr := ctx.Bot().Edit(sentMsg, emptyMsg); editErr != nil {
				h.logger.Error("Failed to send empty result message", zap.Error(editErr))
			}
			return
		}

		h.logger.Info("Creating meeting record")
		_, err = h.meetingService.CreateMeeting(wpCtx, ctx.Sender().ID, fileId, transcription)
		if err != nil {
			h.logger.Error("Failed to create meeting", zap.Error(err))
			return
		}
		h.logger.Info("Meeting record created successfully")

		h.logger.Info("Calling Gigachat for summarization")
		summary, err := h.gigachatClient.Summarize(wpCtx, transcription)
		if err != nil {
			h.logger.Error("Failed to summarize with Gigachat", zap.Error(err))
			// Send error message to user
			errorMsg := "❌ Не удалось создать краткое содержание. Пожалуйста, попробуйте еще раз."
			if _, editErr := ctx.Bot().Edit(sentMsg, errorMsg); editErr != nil {
				h.logger.Error("Failed to send error message", zap.Error(editErr))
			}
			return
		}
		h.logger.Info("Gigachat summarization completed", zap.String("summary", summary))

		h.logger.Info("Updating meeting with summary")
		err = h.meetingService.UpdateMeetingSummary(wpCtx, ctx.Sender().ID, fileId, summary)
		if err != nil {
			h.logger.Error("Failed to update meeting with summary", zap.Error(err))
			return
		}
		h.logger.Info("Meeting updated with summary successfully")

		h.logger.Info("Editing message with summary")
		if _, err := ctx.Bot().Edit(sentMsg, summary); err != nil {
			h.logger.Error("Failed to edit message", zap.Error(err))
			return
		}
		h.logger.Info("Message edited successfully")
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

	processingMsg := "⏳ Обрабатываю аудиофайл..."
	sentMsg, err := ctx.Bot().Send(ctx.Chat(), processingMsg)
	if err != nil {
		return err
	}

	h.wp.Submit(func() {
		h.logger.Info("Starting audio file processing in worker pool")

		wpCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		fileId := audio.FileID
		h.logger.Info("Getting audio file info by ID", zap.String("file_id", fileId))
		f, err := ctx.Bot().FileByID(fileId)
		if err != nil {
			h.logger.Error("Failed to get audio file info by ID", zap.Error(err))
			return
		}

		h.logger.Info("Downloading audio file")
		rc, err := ctx.Bot().File(&f)
		if err != nil {
			h.logger.Error("Failed download audio file", zap.Error(err))
			return
		}

		defer func() {
			h.logger.Info("Closing audio file reader")
			_ = rc.Close()
		}()

		h.logger.Info("Starting transcription")
		transcription, err := h.transcriberClient.TranscribeFile(wpCtx, rc, "MP3")
		if err != nil {
			h.logger.Error("Failed to transcribe audio", zap.Error(err))
			// Send error message to user
			errorMsg := "❌ Не удалось распознать аудиофайл. Пожалуйста, попробуйте еще раз."
			if _, editErr := ctx.Bot().Edit(sentMsg, errorMsg); editErr != nil {
				h.logger.Error("Failed to send error message", zap.Error(editErr))
			}
			return
		}
		h.logger.Info("Transcription completed", zap.String("transcription", transcription))

		if transcription == "" {
			h.logger.Warn("Transcription returned empty result")
			emptyMsg := "⚠️ Аудиофайл распознан, но текст не найден. Возможно, файл слишком короткий или содержит только шум."
			if _, editErr := ctx.Bot().Edit(sentMsg, emptyMsg); editErr != nil {
				h.logger.Error("Failed to send empty result message", zap.Error(editErr))
			}
			return
		}

		h.logger.Info("Creating meeting record")
		_, err = h.meetingService.CreateMeeting(wpCtx, ctx.Sender().ID, fileId, transcription)
		if err != nil {
			h.logger.Error("Failed to create meeting", zap.Error(err))
			return
		}
		h.logger.Info("Meeting record created successfully")

		h.logger.Info("Calling Gigachat for summarization")
		summary, err := h.gigachatClient.Summarize(wpCtx, transcription)
		if err != nil {
			h.logger.Error("Failed to summarize with Gigachat", zap.Error(err))
			// Send error message to user
			errorMsg := "❌ Не удалось создать краткое содержание. Пожалуйста, попробуйте еще раз."
			if _, editErr := ctx.Bot().Edit(sentMsg, errorMsg); editErr != nil {
				h.logger.Error("Failed to send error message", zap.Error(editErr))
			}
			return
		}
		h.logger.Info("Gigachat summarization completed", zap.String("summary", summary))

		h.logger.Info("Updating meeting with summary")
		err = h.meetingService.UpdateMeetingSummary(wpCtx, ctx.Sender().ID, fileId, summary)
		if err != nil {
			h.logger.Error("Failed to update meeting with summary", zap.Error(err))
			return
		}
		h.logger.Info("Meeting updated with summary successfully")

		h.logger.Info("Editing message with summary")
		if _, err := ctx.Bot().Edit(sentMsg, summary); err != nil {
			h.logger.Error("Failed to edit message", zap.Error(err))
			return
		}
		h.logger.Info("Message edited successfully")
	})

	return nil
}
