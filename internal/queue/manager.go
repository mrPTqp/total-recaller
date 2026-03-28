package queue

import (
	"context"
	"sync"

	"github.com/mrPTqp/total-recaller/internal/config"
)

// QueueManager manages all channels for the application
type QueueManager struct {
	// Transcriber channels
	TranscriberTasks   chan TranscriberTask
	TranscriberResults chan TranscriberResult
	
	// LLM channels
	LLMTasks   chan LLMTask
	LLMResults chan LLMResult
	
	// Synchronization
	wg sync.WaitGroup
}

// QueueConfig holds configuration for the queue manager
type QueueConfig struct {
	TaskBufferSize int
}

// NewQueueManager creates a new queue manager with the given configuration
func NewQueueManager(cfg *config.Config) *QueueManager {
	return &QueueManager{
		TranscriberTasks:   make(chan TranscriberTask, cfg.Transcriber.TaskBufferSize),
		TranscriberResults: make(chan TranscriberResult, cfg.Transcriber.ResultBufferSize),
		LLMTasks:           make(chan LLMTask, cfg.LLM.TaskBufferSize),
		LLMResults:         make(chan LLMResult, cfg.LLM.ResultBufferSize),
	}
}

// StartWorkers starts the worker goroutines for processing tasks
func (qm *QueueManager) StartWorkers(
	ctx context.Context,
	transcriberWorker func(context.Context, *QueueManager),
	llmWorker func(context.Context, *QueueManager),
) {
	// Start transcriber worker
	qm.wg.Add(1)
	go func() {
		defer qm.wg.Done()
		transcriberWorker(ctx, qm)
	}()
	
	// Start LLM worker
	qm.wg.Add(1)
	go func() {
		defer qm.wg.Done()
		llmWorker(ctx, qm)
	}()
}

// Wait waits for all workers to complete
func (qm *QueueManager) Wait() {
	qm.wg.Wait()
}

// Close closes all channels
func (qm *QueueManager) Close() {
	close(qm.TranscriberTasks)
	close(qm.TranscriberResults)
	close(qm.LLMTasks)
	close(qm.LLMResults)
}