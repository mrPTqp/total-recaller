package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/mrPTqp/total-recaller/internal/llm"
	"github.com/mrPTqp/total-recaller/internal/transcriber"
	"go.uber.org/zap"
)

// WorkerManager manages the worker goroutines
type WorkerManager struct {
	logger            *zap.Logger
	transcriberClient *transcriber.TranscriberClient
	llmClient         *llm.LLMClient
}

// NewWorkerManager creates a new worker manager
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

// TranscriberWorker processes transcription tasks from the queue
func (wm *WorkerManager) TranscriberWorker(ctx context.Context, qm *QueueManager) {
	wm.logger.Info("Starting transcriber worker")
	
	for {
		select {
		case <-ctx.Done():
			wm.logger.Info("Transcriber worker shutting down")
			return
			
		case task, ok := <-qm.TranscriberTasks:
			if !ok {
				wm.logger.Info("Transcriber tasks channel closed")
				return
			}
			
			wm.processTranscriberTask(ctx, qm, task)
		}
	}
}

// LLMWorker processes LLM tasks from the queue
func (wm *WorkerManager) LLMWorker(ctx context.Context, qm *QueueManager) {
	wm.logger.Info("Starting LLM worker")
	
	for {
		select {
		case <-ctx.Done():
			wm.logger.Info("LLM worker shutting down")
			return
			
		case task, ok := <-qm.LLMTasks:
			if !ok {
				wm.logger.Info("LLM tasks channel closed")
				return
			}
			
			wm.processLLMTask(ctx, qm, task)
		}
	}
}

// EmbeddingWorker processes embedding tasks from the queue
func (wm *WorkerManager) EmbeddingWorker(ctx context.Context, qm *QueueManager) {
	wm.logger.Info("Starting embedding worker")
	
	for {
		select {
		case <-ctx.Done():
			wm.logger.Info("Embedding worker shutting down")
			return
			
		case task, ok := <-qm.EmbeddingTasks:
			if !ok {
				wm.logger.Info("Embedding tasks channel closed")
				return
			}
			
			wm.processEmbeddingTask(ctx, qm, task)
		}
	}
}

// processTranscriberTask processes a single transcription task
func (wm *WorkerManager) processTranscriberTask(ctx context.Context, qm *QueueManager, task TranscriberTask) {
	wm.logger.Info("Processing transcriber task",
		zap.String("task_id", task.ID),
		zap.Int64("user_id", task.UserID),
		zap.String("file_id", task.FileID),
		zap.String("audio_format", task.AudioFormat),
	)

	// Perform transcription
	transcription, err := wm.transcriberClient.TranscribeFile(ctx, task.AudioData, task.AudioFormat)
	if err != nil {
		wm.logger.Error("Failed to transcribe audio",
			zap.String("task_id", task.ID),
			zap.Error(err))
		
		// Send error result
		result := TranscriberResult{
			TaskID:      task.ID,
			UserID:      task.UserID,
			FileID:      task.FileID,
			Transcription: "",
			Error:       err,
			CreatedAt:   time.Now(),
		}
		
		select {
		case qm.TranscriberResults <- result:
			wm.logger.Info("Sent transcriber error result",
				zap.String("task_id", task.ID))
		case <-ctx.Done():
			wm.logger.Info("Context cancelled while sending transcriber error result",
				zap.String("task_id", task.ID))
		}
		return
	}
	
	wm.logger.Info("Transcription completed successfully",
		zap.String("task_id", task.ID),
		zap.Int("transcription_length", len(transcription)))
	
	// Send successful result
	result := TranscriberResult{
		TaskID:      task.ID,
		UserID:      task.UserID,
		FileID:      task.FileID,
		Transcription: transcription,
		Error:       nil,
		CreatedAt:   time.Now(),
	}
	
	select {
	case qm.TranscriberResults <- result:
		wm.logger.Info("Sent transcriber result",
			zap.String("task_id", task.ID))
	case <-ctx.Done():
		wm.logger.Info("Context cancelled while sending transcriber result",
			zap.String("task_id", task.ID))
	}
}

// processLLMTask processes a single LLM task
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
			zap.Error(err))
		
		// Send error result
		result := LLMResult{
			TaskID:    task.ID,
			UserID:    task.UserID,
			Response:  "",
			Error:     err,
			CreatedAt: time.Now(),
		}
		
		select {
		case qm.LLMResults <- result:
			wm.logger.Info("Sent LLM error result",
				zap.String("task_id", task.ID))
		case <-ctx.Done():
			wm.logger.Info("Context cancelled while sending LLM error result",
				zap.String("task_id", task.ID))
		}
		return
	}
	
	wm.logger.Info("LLM task completed successfully",
		zap.String("task_id", task.ID),
		zap.Int("response_length", len(response)))
	
	// Send successful result
	result := LLMResult{
		TaskID:    task.ID,
		UserID:    task.UserID,
		Response:  response,
		Error:     nil,
		CreatedAt: time.Now(),
	}
	
	select {
	case qm.LLMResults <- result:
		wm.logger.Info("Sent LLM result",
			zap.String("task_id", task.ID))
	case <-ctx.Done():
		wm.logger.Info("Context cancelled while sending LLM result",
			zap.String("task_id", task.ID))
	}
}

// processEmbeddingTask processes a single embedding task
func (wm *WorkerManager) processEmbeddingTask(ctx context.Context, qm *QueueManager, task EmbeddingTask) {
	wm.logger.Info("Processing embedding task",
		zap.String("task_id", task.ID),
		zap.Int64("user_id", task.UserID),
		zap.String("file_id", task.FileID),
		zap.Int("text_length", len(task.Text)),
	)
	
	// Generate embedding using LLM client
	embedding, err := wm.llmClient.GenerateEmbedding(ctx, task.Text)
	if err != nil {
		wm.logger.Error("Failed to generate embedding",
			zap.String("task_id", task.ID),
			zap.Error(err))
		
		// Send error result
		result := EmbeddingResult{
			TaskID:    task.ID,
			UserID:    task.UserID,
			Embedding: nil,
			Error:     err,
			CreatedAt: time.Now(),
		}
		
		select {
		case qm.EmbeddingResults <- result:
			wm.logger.Info("Sent embedding error result",
				zap.String("task_id", task.ID))
		case <-ctx.Done():
			wm.logger.Info("Context cancelled while sending embedding error result",
				zap.String("task_id", task.ID))
		}
		return
	}
	
	wm.logger.Info("Embedding generated successfully",
		zap.String("task_id", task.ID),
		zap.Int("embedding_dimensions", len(embedding)))
	
	// Send successful result
	result := EmbeddingResult{
		TaskID:    task.ID,
		UserID:    task.UserID,
		Embedding: embedding,
		Error:     nil,
		CreatedAt: time.Now(),
	}
	
	select {
	case qm.EmbeddingResults <- result:
		wm.logger.Info("Sent embedding result",
			zap.String("task_id", task.ID))
	case <-ctx.Done():
		wm.logger.Info("Context cancelled while sending embedding result",
			zap.String("task_id", task.ID))
	}
}
