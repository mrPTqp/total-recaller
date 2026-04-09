package queue

import (
	"context"
	"sync"

	"github.com/mrPTqp/total-recaller/internal/config"
)

type QueueManager struct {
	// Каналы задач - используются хендлерами для отправки задач воркерам
	TranscriberTasks chan TranscriberTask
	LLMTasks         chan LLMTask
	EmbeddingTasks   chan EmbeddingTask

	// Персональные каналы для ожидания результатов по taskID
	// Используем sync.Map для безопасного конкурентного доступа
	transcriberWaiters sync.Map // map[string]chan TranscriberResult
	llmWaiters         sync.Map // map[string]chan LLMResult
	embeddingWaiters   sync.Map // map[string]chan EmbeddingResult

	wg sync.WaitGroup
}

func NewQueueManager(cfg *config.Config) *QueueManager {
	return &QueueManager{
		TranscriberTasks: make(chan TranscriberTask, cfg.Transcriber.TaskBufferSize),
		LLMTasks:         make(chan LLMTask, cfg.LLM.TaskBufferSize),
		EmbeddingTasks:   make(chan EmbeddingTask, cfg.LLM.TaskBufferSize),
	}
}

// RegisterTranscriberWaiter регистрирует канал для ожидания результата транскрибации
func (qm *QueueManager) RegisterTranscriberWaiter(taskID string) chan TranscriberResult {
	resultChan := make(chan TranscriberResult, 1)
	qm.transcriberWaiters.Store(taskID, resultChan)
	return resultChan
}

// UnregisterTranscriberWaiter удаляет канал ожидания результата транскрибации
func (qm *QueueManager) UnregisterTranscriberWaiter(taskID string) {
	qm.transcriberWaiters.Delete(taskID)
}

// RegisterLLMWaiter регистрирует канал для ожидания результата LLM
func (qm *QueueManager) RegisterLLMWaiter(taskID string) chan LLMResult {
	resultChan := make(chan LLMResult, 1)
	qm.llmWaiters.Store(taskID, resultChan)
	return resultChan
}

// UnregisterLLMWaiter удаляет канал ожидания результата LLM
func (qm *QueueManager) UnregisterLLMWaiter(taskID string) {
	qm.llmWaiters.Delete(taskID)
}

// RegisterEmbeddingWaiter регистрирует канал для ожидания результата эмбеддинга
func (qm *QueueManager) RegisterEmbeddingWaiter(taskID string) chan EmbeddingResult {
	resultChan := make(chan EmbeddingResult, 1)
	qm.embeddingWaiters.Store(taskID, resultChan)
	return resultChan
}

// UnregisterEmbeddingWaiter удаляет канал ожидания результата эмбеддинга
func (qm *QueueManager) UnregisterEmbeddingWaiter(taskID string) {
	qm.embeddingWaiters.Delete(taskID)
}

func (qm *QueueManager) StartWorkers(
	ctx context.Context,
	transcriberWorker func(context.Context, *QueueManager),
	llmWorker func(context.Context, *QueueManager),
	embeddingWorker func(context.Context, *QueueManager),
) {
	qm.wg.Add(3)

	go func() {
		defer qm.wg.Done()
		transcriberWorker(ctx, qm)
	}()

	go func() {
		defer qm.wg.Done()
		llmWorker(ctx, qm)
	}()

	go func() {
		defer qm.wg.Done()
		embeddingWorker(ctx, qm)
	}()
}

func (qm *QueueManager) Wait() {
	qm.wg.Wait()
}