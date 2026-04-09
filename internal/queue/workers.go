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

		// Отправляем результат через персональный канал ожидающей горутины
		wm.sendTranscriberResult(ctx, qm, task.ID, result)
		return
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

	// Отправляем результат через персональный канал ожидающей горутины
	wm.sendTranscriberResult(ctx, qm, task.ID, result)
}

// sendTranscriberResult отправляет результат транскрибации через персональный канал
func (wm *WorkerManager) sendTranscriberResult(ctx context.Context, qm *QueueManager, taskID string, result TranscriberResult) {
	if waiter, ok := qm.transcriberWaiters.LoadAndDelete(taskID); ok {
		resultChan := waiter.(chan TranscriberResult)
		select {
		case resultChan <- result:
			wm.logger.Info("Sent transcriber result to waiter",
				zap.String("task_id", taskID),
			)
		case <-ctx.Done():
			wm.logger.Info("Context cancelled while sending transcriber result to waiter",
				zap.String("task_id", taskID),
			)
		}
	} else {
		wm.logger.Warn("No waiter found for transcriber result",
			zap.String("task_id", taskID),
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

		// Отправляем результат через персональный канал ожидающей горутины
		wm.sendLLMResult(ctx, qm, task.ID, result)
		return
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

	// Отправляем результат через персональный канал ожидающей горутины
	wm.sendLLMResult(ctx, qm, task.ID, result)
}

// sendLLMResult отправляет результат LLM через персональный канал
func (wm *WorkerManager) sendLLMResult(ctx context.Context, qm *QueueManager, taskID string, result LLMResult) {
	if waiter, ok := qm.llmWaiters.LoadAndDelete(taskID); ok {
		resultChan := waiter.(chan LLMResult)
		select {
		case resultChan <- result:
			wm.logger.Info("Sent LLM result to waiter",
				zap.String("task_id", taskID),
			)
		case <-ctx.Done():
			wm.logger.Info("Context cancelled while sending LLM result to waiter",
				zap.String("task_id", taskID),
			)
		}
	} else {
		wm.logger.Warn("No waiter found for LLM result",
			zap.String("task_id", taskID),
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

		// Отправляем результат через персональный канал ожидающей горутины
		wm.sendEmbeddingResult(ctx, qm, task.ID, result)
		return
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

	// Отправляем результат через персональный канал ожидающей горутины
	wm.sendEmbeddingResult(ctx, qm, task.ID, result)
}

// sendEmbeddingResult отправляет результат эмбеддинга через персональный канал
func (wm *WorkerManager) sendEmbeddingResult(ctx context.Context, qm *QueueManager, taskID string, result EmbeddingResult) {
	if waiter, ok := qm.embeddingWaiters.LoadAndDelete(taskID); ok {
		resultChan := waiter.(chan EmbeddingResult)
		select {
		case resultChan <- result:
			wm.logger.Info("Sent embedding result to waiter",
				zap.String("task_id", taskID),
			)
		case <-ctx.Done():
			wm.logger.Info("Context cancelled while sending embedding result to waiter",
				zap.String("task_id", taskID),
			)
		}
	} else {
		wm.logger.Warn("No waiter found for embedding result",
			zap.String("task_id", taskID),
		)
	}
}
