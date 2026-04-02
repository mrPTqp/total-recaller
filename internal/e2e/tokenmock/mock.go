package tokenmock

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// TokenMock represents a mock token server that simulates
// the Sberbank OAuth token endpoint (ngw.devices.sberbank.ru:9443/api/v2/oauth).
// It can serve tokens for both transcriber and LLM scopes.
type TokenMock struct {
	server *http.Server
	addr   string
	logger *zap.Logger

	// Token management
	transcriberToken string
	llmToken         string
	tokenMu          sync.RWMutex

	// Statistics
	stats *MockStats

	// Running state
	running bool
	wg      sync.WaitGroup
}

type MockStats struct {
	mu            sync.RWMutex
	TokenRequests int `json:"token_requests"`
}

// NewTokenMock creates a new mock token server.
// The addr parameter can be ":0" to automatically select an available port.
func NewTokenMock(addr string, logger *zap.Logger) (*TokenMock, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	mock := &TokenMock{
		logger:           logger,
		addr:             addr,
		stats:            &MockStats{},
		transcriberToken: "mock-transcriber-access-token",
		llmToken:         "mock-llm-access-token",
	}

	mock.initServer()

	return mock, nil
}

func (m *TokenMock) initServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/oauth", m.handleTokenRequest)

	m.server = &http.Server{
		Addr:    m.addr,
		Handler: mux,
	}
}

// StartAsync starts the token server asynchronously
func (m *TokenMock) StartAsync() error {
	listener, err := net.Listen("tcp", m.addr)
	if err != nil {
		return fmt.Errorf("failed to create token server listener: %w", err)
	}
	m.addr = listener.Addr().String()

	m.running = true
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		if err := m.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			m.logger.Error("Token mock server error", zap.Error(err))
		}
	}()

	m.logger.Info("Token mock server started", zap.String("addr", m.addr))

	return nil
}

// Stop stops the token server
func (m *TokenMock) Stop() error {
	m.running = false
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := m.server.Shutdown(ctx)
	m.wg.Wait()
	return err
}

// URL returns the base URL for the token server
func (m *TokenMock) URL() string {
	return fmt.Sprintf("http://%s", m.addr)
}

// SetTranscriberToken allows setting a custom access token for transcriber
func (m *TokenMock) SetTranscriberToken(token string) {
	m.tokenMu.Lock()
	m.transcriberToken = token
	m.tokenMu.Unlock()
}

// SetLLMToken allows setting a custom access token for LLM
func (m *TokenMock) SetLLMToken(token string) {
	m.tokenMu.Lock()
	m.llmToken = token
	m.tokenMu.Unlock()
}

// GetTranscriberToken returns the current transcriber access token
func (m *TokenMock) GetTranscriberToken() string {
	m.tokenMu.RLock()
	defer m.tokenMu.RUnlock()
	return m.transcriberToken
}

// GetLLMToken returns the current LLM access token
func (m *TokenMock) GetLLMToken() string {
	m.tokenMu.RLock()
	defer m.tokenMu.RUnlock()
	return m.llmToken
}

// GetStats returns current statistics from the mock server
func (m *TokenMock) GetStats() map[string]any {
	m.stats.mu.RLock()
	defer m.stats.mu.RUnlock()

	return map[string]any{
		"token_requests": m.stats.TokenRequests,
	}
}

// ResetStats resets all statistics counters to zero
func (m *TokenMock) ResetStats() {
	m.stats.mu.Lock()
	m.stats.TokenRequests = 0
	m.stats.mu.Unlock()
}

func (m *TokenMock) handleTokenRequest(w http.ResponseWriter, r *http.Request) {
	m.stats.mu.Lock()
	m.stats.TokenRequests++
	m.stats.mu.Unlock()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse form data
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	scope := r.FormValue("scope")
	if scope == "" {
		http.Error(w, "Missing scope parameter", http.StatusBadRequest)
		return
	}

	m.logger.Debug("Token request received", zap.String("scope", scope))

	// Return appropriate token based on scope
	var token string
	switch scope {
	case "SALUTE_SPEECH_PERS":
		token = m.GetTranscriberToken()
	case "GIGACHAT_API_PERS":
		token = m.GetLLMToken()
	default:
		token = "mock-unknown-token"
	}

	response := TokenResponse{
		AccessToken: token,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// TokenResponse represents the OAuth token response
type TokenResponse struct {
	AccessToken string `json:"access_token"`
}