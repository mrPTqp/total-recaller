package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/mrPTqp/total-recaller/internal/llm"
	"github.com/mrPTqp/total-recaller/internal/transcriber"
	"go.uber.org/zap"
)

type WorkerManager struct {
	logger            *zap.Logger
	transcriberClient *transcriber.TranscriberClient
	llmClient         *llm.LLMClient
}

func NewWorkerManager(
	logger *zap.Logger,
	transcriberClient *transcriber.TranscriberClient,
	llmClient *llm.LLMClient,
) *WorkerManager {
	return &WorkerManager{
		logger:            logger,
		transcriberClient: transcriberClient,
		llmClient:         llmClient,
	}
}

func (wm *WorkerManager) TranscriberWorker(ctx context.Context, qm *QueueManager) {
	for {
		select {
		case <-ctx.Done():
			return
		case task := <-qm.TranscriberTasks:
			wm.processTranscriberTask(ctx, qm, task)
		}
	}
}

func (wm *WorkerManager) LLMWorker(ctx context.Context, qm *QueueManager) {
	for {
		select {
		case <-ctx.Done():
			return
		case task := <-qm.LLMTasks:
			wm.processLLMTask(ctx, qm, task)
		}
	}
}

func (wm *WorkerManager) EmbeddingWorker(ctx context.Context, qm *QueueManager) {
	for {
		select {
		case <-ctx.Done():
			return
		case task := <-qm.EmbeddingTasks:
			wm.processEmbeddingTask(ctx, qm, task)
		}
	}
}

func (wm *WorkerManager) processTranscriberTask(ctx context.Context, qm *QueueManager, task TranscriberTask) {
	wm.logger.Info("Processing transcriber task",
		zap.String("task_id", task.ID),
		zap.Int64("user_id", task.UserID),
		zap.String("file_id", task.FileID),
		zap.String("audio_format", task.AudioFormat),
	)

	transcription, err := wm.transcriberClient.TranscribeFile(ctx, task.AudioData, task.AudioFormat)
	if err != nil {
		wm.logger.Error("Failed to transcribe audio",
			zap.String("task_id", task.ID),
			zap.Error(err),
		)

		result := TranscriberResult{
			TaskID:        task.ID,
			UserID:        task.UserID,
			FileID:        task.FileID,
			Transcription: "",
			Error:         err,
			CreatedAt:     time.Now(),
		}

		select {
		case <-ctx.Done():
			wm.logger.Info("Context cancelled while sending transcriber error result",
				zap.String("task_id", task.ID),
			)
			return
		case qm.TranscriberResults <- result:
			wm.logger.Info("Sent transcriber error result",
				zap.String("task_id", task.ID),
			)
			return
		}
	}

	wm.logger.Info("Transcription completed successfully",
		zap.String("task_id", task.ID),
		zap.Int("transcription_length", len(transcription)),
	)

	result := TranscriberResult{
		TaskID:        task.ID,
		UserID:        task.UserID,
		FileID:        task.FileID,
		Transcription: transcription,
		Error:         nil,
		CreatedAt:     time.Now(),
	}

	select {
	case <-ctx.Done():
		wm.logger.Info("Context cancelled while sending transcriber result",
			zap.String("task_id", task.ID),
		)
		return
	case qm.TranscriberResults <- result:
		wm.logger.Info("Sent transcriber result",
			zap.String("task_id", task.ID),
		)
	}
}

func (wm *WorkerManager) processLLMTask(ctx context.Context, qm *QueueManager, task LLMTask) {
	wm.logger.Info("Processing LLM task",
		zap.String("task_id", task.ID),
		zap.Int64("user_id", task.UserID),
		zap.String("task_type", string(task.TaskType)),
		zap.Int("text_length", len(task.Text)),
	)

	var response string
	var err error

	switch task.TaskType {
	case LLMTaskTypeSummarize:
		response, err = wm.llmClient.Summarize(ctx, task.Text)
	case LLMTaskTypeChat:
		response, err = wm.llmClient.Chat(ctx, task.Query)
	default:
		err = fmt.Errorf("unknown task type: %s", task.TaskType)
	}

	if err != nil {
		wm.logger.Error("Failed to process LLM task",
			zap.String("task_id", task.ID),
			zap.Error(err),
		)

		result := LLMResult{
			TaskID:    task.ID,
			UserID:    task.UserID,
			Response:  "",
			Error:     err,
			CreatedAt: time.Now(),
		}

		select {
		case <-ctx.Done():
			wm.logger.Info("Context cancelled while sending LLM error result",
				zap.String("task_id", task.ID),
			)
			return
		case qm.LLMResults <- result:
			wm.logger.Info("Sent LLM error result",
				zap.String("task_id", task.ID),
			)
			return
		}
	}

	wm.logger.Info("LLM task completed successfully",
		zap.String("task_id", task.ID),
		zap.Int("response_length", len(response)),
	)

	result := LLMResult{
		TaskID:    task.ID,
		UserID:    task.UserID,
		Response:  response,
		Error:     nil,
		CreatedAt: time.Now(),
	}

	select {
	case <-ctx.Done():
		wm.logger.Info("Context cancelled while sending LLM result",
			zap.String("task_id", task.ID),
		)
		return
	case qm.LLMResults <- result:
		wm.logger.Info("Sent LLM result",
			zap.String("task_id", task.ID),
		)
	}
}

func (wm *WorkerManager) processEmbeddingTask(ctx context.Context, qm *QueueManager, task EmbeddingTask) {
	wm.logger.Info("Processing embedding task",
		zap.String("task_id", task.ID),
		zap.Int64("user_id", task.UserID),
		zap.String("file_id", task.FileID),
		zap.Int("text_length", len(task.Text)),
	)

	embedding, err := wm.llmClient.GenerateEmbedding(ctx, task.Text)
	if err != nil {
		wm.logger.Error("Failed to generate embedding",
			zap.String("task_id", task.ID),
			zap.Error(err),
		)

		result := EmbeddingResult{
			TaskID:    task.ID,
			UserID:    task.UserID,
			Embedding: nil,
			Error:     err,
			CreatedAt: time.Now(),
		}

		select {
		case <-ctx.Done():
			wm.logger.Info("Context cancelled while sending embedding error result",
				zap.String("task_id", task.ID),
			)
			return
		case qm.EmbeddingResults <- result:
			wm.logger.Info("Sent embedding error result",
				zap.String("task_id", task.ID),
			)
			return
		}
	}

	wm.logger.Info("Embedding generated successfully",
		zap.String("task_id", task.ID),
		zap.Int("embedding_dimensions", len(embedding)),
	)

	result := EmbeddingResult{
		TaskID:    task.ID,
		UserID:    task.UserID,
		Embedding: embedding,
		Error:     nil,
		CreatedAt: time.Now(),
	}

	select {
	case <-ctx.Done():
		wm.logger.Info("Context cancelled while sending embedding result",
			zap.String("task_id", task.ID),
		)
		return
	case qm.EmbeddingResults <- result:
		wm.logger.Info("Sent embedding result",
			zap.String("task_id", task.ID),
		)
	}
}
