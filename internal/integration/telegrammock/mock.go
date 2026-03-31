package telegrammock

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// TelegramAPIMock represents a mock Telegram Bot API server.
// It implements the getUpdates long polling mechanism and provides
// a control API for injecting events during tests.
type TelegramAPIMock struct {
	token  string
	logger *zap.Logger
	server *http.Server
	addr   string

	// Event queue for getUpdates
	events   []Update
	eventsMu sync.Mutex
	newEvent chan struct{}

	// File storage for audio/voice files
	fileStorage map[string]string // fileID -> file path
	filesMu     sync.RWMutex

	// Statistics
	stats *MockStats

	// Running state
	running bool
	wg      sync.WaitGroup
}

// NewTelegramAPIMock creates a new mock Telegram Bot API server.
// The addr parameter can be ":0" to automatically select an available port.
func NewTelegramAPIMock(token string, addr string, logger *zap.Logger) (*TelegramAPIMock, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	mock := &TelegramAPIMock{
		token:       token,
		logger:      logger,
		addr:        addr,
		fileStorage: make(map[string]string),
		newEvent:    make(chan struct{}, 1),
		stats: &MockStats{
			EventsByType: make(map[string]int),
		},
	}

	mock.initHTTPServer()

	return mock, nil
}

// initHTTPServer sets up the HTTP server with all required endpoints
func (m *TelegramAPIMock) initHTTPServer() {
	mux := http.NewServeMux()

	// Telegram Bot API endpoints
	mux.HandleFunc(fmt.Sprintf("/bot%s/", m.token), m.handleBotAPI)
	mux.HandleFunc(fmt.Sprintf("/bot%s/getUpdates", m.token), m.handleGetUpdates)
	mux.HandleFunc(fmt.Sprintf("/bot%s/getFile", m.token), m.handleGetFile)
	mux.HandleFunc(fmt.Sprintf("/bot%s/sendMessage", m.token), m.handleSendMessage)
	mux.HandleFunc(fmt.Sprintf("/bot%s/editMessageText", m.token), m.handleEditMessageText)
	mux.HandleFunc(fmt.Sprintf("/bot%s/setMyCommands", m.token), m.handleSetMyCommands)

	// File download endpoint (Telegram serves files at /file/<file_path>)
	mux.HandleFunc("/file/", m.handleFileDownload)

	// Control API endpoints for test management
	mux.HandleFunc("/control/inject/text", m.handleInjectText)
	mux.HandleFunc("/control/inject/voice", m.handleInjectVoice)
	mux.HandleFunc("/control/inject/audio", m.handleInjectAudio)
	mux.HandleFunc("/control/inject/command", m.handleInjectCommand)
	mux.HandleFunc("/control/stats", m.handleGetStats)
	mux.HandleFunc("/control/reset", m.handleResetStats)
	mux.HandleFunc("/control/health", m.handleHealthCheck)

	m.server = &http.Server{
		Addr:    m.addr,
		Handler: mux,
	}
}

func (m *TelegramAPIMock) Start() error {
	listener, err := net.Listen("tcp", m.addr)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	m.addr = listener.Addr().String()
	m.running = true

	m.logger.Info("Starting Telegram API mock server", zap.String("addr", m.addr))
	return m.server.Serve(listener)
}

func (m *TelegramAPIMock) StartAsync() error {
	listener, err := net.Listen("tcp", m.addr)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	m.addr = listener.Addr().String()
	m.running = true

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		if err := m.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			m.logger.Error("Mock server error", zap.Error(err))
		}
	}()

	m.logger.Info("Telegram API mock server started", zap.String("addr", m.addr))
	return nil
}

func (m *TelegramAPIMock) Stop() error {
	m.running = false
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := m.server.Shutdown(ctx)
	m.wg.Wait()
	return err
}

func (m *TelegramAPIMock) Addr() string {
	return m.addr
}

func (m *TelegramAPIMock) BaseURL() string {
	return fmt.Sprintf("http://%s", m.addr)
}

func (m *TelegramAPIMock) queueUpdate(update Update) {
	m.eventsMu.Lock()
	m.events = append(m.events, update)
	m.eventsMu.Unlock()

	// Notify getUpdates that there's a new event
	select {
	case m.newEvent <- struct{}{}:
	default:
		// Channel already has a notification pending
	}

	m.logger.Debug("Queued update",
		zap.Int("update_id", update.ID),
		zap.Int64("user_id", update.Message.Sender.ID),
	)
}

func (m *TelegramAPIMock) generateUpdateID() uint64 {
	return uint64(time.Now().UnixNano())
}

// InjectTextMessage injects a text message update into the mock server.
func (m *TelegramAPIMock) InjectTextMessage(userID int64, text string) {
	update := m.createTextMessageUpdate(userID, text)
	m.queueUpdate(update)
}

// InjectVoiceMessage injects a voice message update into the mock server.
func (m *TelegramAPIMock) InjectVoiceMessage(userID int64, filePath string) error {
	update, err := m.createVoiceMessageUpdate(userID, filePath)
	if err != nil {
		return err
	}
	m.queueUpdate(update)
	return nil
}

// InjectAudioMessage injects an audio message update into the mock server.
func (m *TelegramAPIMock) InjectAudioMessage(userID int64, filePath string) error {
	update, err := m.createAudioMessageUpdate(userID, filePath)
	if err != nil {
		return err
	}
	m.queueUpdate(update)
	return nil
}

// InjectAudioMessageWithFile injects an audio message update with a specific filename.
func (m *TelegramAPIMock) InjectAudioMessageWithFile(userID int64, filePath string, fileName string) error {
	update, err := m.createAudioMessageUpdateWithFileName(userID, filePath, fileName)
	if err != nil {
		return err
	}
	m.queueUpdate(update)
	return nil
}

// InjectCommand injects a command update into the mock server.
func (m *TelegramAPIMock) InjectCommand(userID int64, command string) {
	update := m.createCommandUpdate(userID, command)
	m.queueUpdate(update)
}

// GetStats returns current statistics from the mock server.
func (m *TelegramAPIMock) GetStats() map[string]any {
	m.stats.mu.RLock()
	defer m.stats.mu.RUnlock()

	return map[string]any{
		"updates_sent":     m.stats.UpdatesSent,
		"files_served":     m.stats.FilesServed,
		"polling_requests": m.stats.PollingRequests,
		"events_by_type":   m.stats.EventsByType,
	}
}

// ResetStats resets all statistics counters to zero.
func (m *TelegramAPIMock) ResetStats() {
	m.stats.mu.Lock()
	m.stats.UpdatesSent = 0
	m.stats.FilesServed = 0
	m.stats.EventsByType = make(map[string]int)
	m.stats.mu.Unlock()
}