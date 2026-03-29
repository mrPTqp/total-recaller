package queue

import (
	"context"
	"sync"

	"github.com/mrPTqp/total-recaller/internal/config"
)

type QueueManager struct {
	TranscriberTasks   chan TranscriberTask
	TranscriberResults chan TranscriberResult
	LLMTasks           chan LLMTask
	LLMResults         chan LLMResult
	EmbeddingTasks     chan EmbeddingTask
	EmbeddingResults   chan EmbeddingResult

	wg sync.WaitGroup
}

func NewQueueManager(cfg *config.Config) *QueueManager {
	return &QueueManager{
		TranscriberTasks:   make(chan TranscriberTask, cfg.Transcriber.TaskBufferSize),
		TranscriberResults: make(chan TranscriberResult, cfg.Transcriber.ResultBufferSize),
		LLMTasks:           make(chan LLMTask, cfg.LLM.TaskBufferSize),
		LLMResults:         make(chan LLMResult, cfg.LLM.ResultBufferSize),
		EmbeddingTasks:     make(chan EmbeddingTask, cfg.LLM.TaskBufferSize),
		EmbeddingResults:   make(chan EmbeddingResult, cfg.LLM.ResultBufferSize),
	}
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
