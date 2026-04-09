package transcribermock

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
)

// TranscriberMock represents a mock transcriber server that simulates
// the Salute Speech API including transcription operations.
type TranscriberMock struct {
	server *http.Server
	addr   string
	logger *zap.Logger

	// File storage for uploaded audio files
	fileStorage map[string][]byte
	filesMu     sync.RWMutex

	// Task management
	tasks   map[string]*transcriptionTask
	tasksMu sync.RWMutex

	// Statistics
	stats *MockStats

	// Running state
	running bool
	wg      sync.WaitGroup
}

// NewTranscriberMock creates a new mock transcriber server.
// The addr parameter can be ":0" to automatically select an available port.
func NewTranscriberMock(addr string, logger *zap.Logger) (*TranscriberMock, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	mock := &TranscriberMock{
		logger:      logger,
		addr:        addr,
		fileStorage: make(map[string][]byte),
		tasks:       make(map[string]*transcriptionTask),
		stats:       &MockStats{},
	}

	mock.initServer()

	return mock, nil
}

func (m *TranscriberMock) initServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/rest/v1/data:upload", m.handleFileUpload)
	mux.HandleFunc("/rest/v1/speech:async_recognize", m.handleRecognize)
	mux.HandleFunc("/rest/v1/task:get", m.handleTaskStatus)
	mux.HandleFunc("/rest/v1/data:download", m.handleDownloadResult)

	m.server = &http.Server{
		Addr:    m.addr,
		Handler: mux,
	}
}

// StartAsync starts the API server asynchronously
func (m *TranscriberMock) StartAsync() error {
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

	m.logger.Info("Transcriber mock server started",
		zap.String("api_addr", m.addr))

	return nil
}

// Stop stops the server
func (m *TranscriberMock) Stop() error {
	m.running = false
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := m.server.Shutdown(ctx)
	m.wg.Wait()

	return err
}

// APIURL returns the base URL for the API server
func (m *TranscriberMock) APIURL() string {
	return fmt.Sprintf("http://%s", m.addr)
}

// GetStats returns current statistics from the mock server
func (m *TranscriberMock) GetStats() map[string]any {
	m.stats.mu.RLock()
	defer m.stats.mu.RUnlock()

	return map[string]any{
		"file_uploads":      m.stats.FileUploads,
		"recognition_tasks": m.stats.RecognitionTasks,
		"status_checks":     m.stats.StatusChecks,
		"result_downloads":  m.stats.ResultDownloads,
	}
}

// ResetStats resets all statistics counters to zero
func (m *TranscriberMock) ResetStats() {
	m.stats.mu.Lock()
	m.stats.FileUploads = 0
	m.stats.RecognitionTasks = 0
	m.stats.StatusChecks = 0
	m.stats.ResultDownloads = 0
	m.stats.mu.Unlock()
}

// CreateTestAudioFile creates a test audio file with some dummy content
func CreateTestAudioFile(path string) error {
	data := []byte{
		0x4F, 0x67, 0x67, 0x53, 0x20,
		0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00,
		0x01,
		0x4F, 0x70, 0x75, 0x73, 0x48, 0x65, 0x61, 0x64,
		0x01,
		0x01,
		0x00, 0x00,
		0x80, 0x3E, 0x00, 0x00,
		0x00, 0x00,
		0x00,
	}
	return os.WriteFile(path, data, 0644)
}