package llmmock

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// LLMMock represents a mock LLM server that simulates
// the GigaChat API for chat completions and embeddings.
type LLMMock struct {
	server *http.Server
	addr   string
	logger *zap.Logger

	// Statistics
	stats *MockStats

	// Running state
	running bool
	wg      sync.WaitGroup

	// Configurable responses
	lastUserMessage string
	lastPrompt      string
	messagesMu      sync.RWMutex
}

// NewLLMMock creates a new mock LLM server.
// The addr parameter can be ":0" to automatically select an available port.
func NewLLMMock(addr string, logger *zap.Logger) (*LLMMock, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	mock := &LLMMock{
		logger: logger,
		addr:   addr,
		stats:  &MockStats{},
	}

	mock.initServer()

	return mock, nil
}

func (m *LLMMock) initServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/chat/completions", m.handleChatCompletions)
	mux.HandleFunc("/api/v1/embeddings", m.handleEmbeddings)

	m.server = &http.Server{
		Addr:    m.addr,
		Handler: mux,
	}
}

// StartAsync starts the API server asynchronously
func (m *LLMMock) StartAsync() error {
	listener, err := net.Listen("tcp", m.addr)
	if err != nil {
		return fmt.Errorf("failed to create API server listener: %w", err)
	}
	m.addr = listener.Addr().String()

	m.running = true
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		if err := m.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			m.logger.Error("API mock server error", zap.Error(err))
		}
	}()

	m.logger.Info("LLM mock server started",
		zap.String("api_addr", m.addr))

	return nil
}

// Stop stops the server
func (m *LLMMock) Stop() error {
	m.running = false
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := m.server.Shutdown(ctx)
	m.wg.Wait()

	return err
}

// APIURL returns the base URL for the API server
func (m *LLMMock) APIURL() string {
	return fmt.Sprintf("http://%s", m.addr)
}

// GetStats returns current statistics from the mock server
func (m *LLMMock) GetStats() map[string]any {
	m.stats.mu.RLock()
	defer m.stats.mu.RUnlock()

	return map[string]any{
		"chat_requests":      m.stats.ChatRequests,
		"embedding_requests": m.stats.EmbeddingRequests,
	}
}

// ResetStats resets all statistics counters to zero
func (m *LLMMock) ResetStats() {
	m.stats.mu.Lock()
	m.stats.ChatRequests = 0
	m.stats.EmbeddingRequests = 0
	m.stats.mu.Unlock()
}

// GetLastUserMessage returns the last user message received in a chat request
func (m *LLMMock) GetLastUserMessage() string {
	m.messagesMu.RLock()
	defer m.messagesMu.RUnlock()
	return m.lastUserMessage
}

// GetLastPrompt returns the last system prompt received
func (m *LLMMock) GetLastPrompt() string {
	m.messagesMu.RLock()
	defer m.messagesMu.RUnlock()
	return m.lastPrompt
}