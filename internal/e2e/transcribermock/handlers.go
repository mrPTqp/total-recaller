package transcribermock

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/mrPTqp/total-recaller/internal/transcriber"
	"go.uber.org/zap"
)

func (m *TranscriberMock) handleFileUpload(w http.ResponseWriter, r *http.Request) {
	m.stats.mu.Lock()
	m.stats.FileUploads++
	m.stats.mu.Unlock()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get token from environment or use default
	expectedToken := os.Getenv("TOTAL_RECALLER_TRANSCRIBER_TOKEN_MANAGER_CREDENTIALS")
	if expectedToken == "" {
		expectedToken = "test-credentials"
	}

	if !m.verifyAuth(r, expectedToken) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	fileID := fmt.Sprintf("file_%d", time.Now().UnixNano())

	m.filesMu.Lock()
	m.fileStorage[fileID] = body
	m.filesMu.Unlock()

	m.logger.Debug("File uploaded", zap.String("file_id", fileID), zap.Int("size", len(body)))

	response := transcriberUploadResponse{
		Result: struct {
			RequestFileID string `json:"request_file_id"`
		}{
			RequestFileID: fileID,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (m *TranscriberMock) handleRecognize(w http.ResponseWriter, r *http.Request) {
	m.stats.mu.Lock()
	m.stats.RecognitionTasks++
	m.stats.mu.Unlock()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get token from environment or use default
	expectedToken := os.Getenv("TOTAL_RECALLER_TRANSCRIBER_TOKEN_MANAGER_CREDENTIALS")
	if expectedToken == "" {
		expectedToken = "test-credentials"
	}

	if !m.verifyAuth(r, expectedToken) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req transcriberRecognizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	m.filesMu.RLock()
	_, exists := m.fileStorage[req.RequestFileID]
	m.filesMu.RUnlock()

	if !exists {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	taskID := fmt.Sprintf("task_%d", time.Now().UnixNano())
	responseFileID := fmt.Sprintf("response_%d", time.Now().UnixNano())

	m.tasksMu.Lock()
	m.tasks[taskID] = &transcriptionTask{
		status:         "DONE",
		responseFileID: responseFileID,
		fileID:         req.RequestFileID,
		createdAt:      time.Now(),
	}
	m.tasksMu.Unlock()

	m.logger.Debug("Recognition task created", zap.String("task_id", taskID))

	response := transcriberRecognizeResponse{
		Result: struct {
			ID string `json:"id"`
		}{
			ID: taskID,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (m *TranscriberMock) handleTaskStatus(w http.ResponseWriter, r *http.Request) {
	m.stats.mu.Lock()
	m.stats.StatusChecks++
	m.stats.mu.Unlock()

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get token from environment or use default
	expectedToken := os.Getenv("TOTAL_RECALLER_TRANSCRIBER_TOKEN_MANAGER_CREDENTIALS")
	if expectedToken == "" {
		expectedToken = "test-credentials"
	}

	if !m.verifyAuth(r, expectedToken) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		http.Error(w, "Missing task ID", http.StatusBadRequest)
		return
	}

	m.tasksMu.RLock()
	task, exists := m.tasks[taskID]
	m.tasksMu.RUnlock()

	if !exists {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	m.logger.Debug("Task status checked", zap.String("task_id", taskID), zap.String("status", task.status))

	response := transcriberStatusResponse{
		Result: struct {
			Status         string `json:"status"`
			ResponseFileID string `json:"response_file_id"`
		}{
			Status:         task.status,
			ResponseFileID: task.responseFileID,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (m *TranscriberMock) handleDownloadResult(w http.ResponseWriter, r *http.Request) {
	m.stats.mu.Lock()
	m.stats.ResultDownloads++
	m.stats.mu.Unlock()

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get token from environment or use default
	expectedToken := os.Getenv("TOTAL_RECALLER_TRANSCRIBER_TOKEN_MANAGER_CREDENTIALS")
	if expectedToken == "" {
		expectedToken = "test-credentials"
	}

	if !m.verifyAuth(r, expectedToken) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	responseFileID := r.URL.Query().Get("response_file_id")
	if responseFileID == "" {
		http.Error(w, "Missing response file ID", http.StatusBadRequest)
		return
	}

	var task *transcriptionTask
	m.tasksMu.RLock()
	for _, t := range m.tasks {
		if t.responseFileID == responseFileID {
			task = t
			break
		}
	}
	m.tasksMu.RUnlock()

	if task == nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	m.filesMu.RLock()
	fileContent, exists := m.fileStorage[task.fileID]
	m.filesMu.RUnlock()

	if !exists {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	m.logger.Debug("Downloading result", zap.String("response_file_id", responseFileID))

	result := m.generateMockTranscription(fileContent)

	// Log the transcription result at INFO level
	var transcriptionText string
	if len(result) > 0 && len(result[0].Results) > 0 {
		transcriptionText = result[0].Results[0].Text
	}
	m.logger.Info("Transcriber returning transcription",
		zap.String("file_id", task.fileID),
		zap.String("transcription", transcriptionText),
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (m *TranscriberMock) verifyAuth(r *http.Request, expectedToken string) bool {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false
	}

	return authHeader == "Bearer " + expectedToken
}

func (m *TranscriberMock) generateMockTranscription(audioContent []byte) transcriber.DownloadResponse {
	duration := len(audioContent) / 16000

	return transcriber.DownloadResponse{
		{
			Results: []transcriber.SaluteTextResult{
				{
					Text:  "Это тестовая транскрипция аудиофайла.",
					Start: transcriber.Timestamp(0),
					End:   transcriber.Timestamp(time.Duration(duration/2) * time.Second),
				},
			},
			Eou: true,
			EmotionsResult: transcriber.EmotionsResult{
				Positive: 0.5,
				Negative: 0.1,
				Neutral:  0.4,
			},
			ProcessedAudioStart: transcriber.Timestamp(0),
			ProcessedAudioEnd:   transcriber.Timestamp(time.Duration(duration) * time.Second),
			BackendInfo: transcriber.BackendInfo{
				ModelName:     "salute",
				ModelVersion:  "1.0.0",
				ServerVersion: "mock-1.0",
			},
			Channel: 0,
			SpeakerInfo: transcriber.SpeakerInfo{
				SpeakerID:             0,
				MainSpeakerConfidence: 0.95,
			},
			EouReason: "timeout",
			PersonIdentity: transcriber.PersonIdentity{
				Age:         "adult",
				Gender:      "unknown",
				AgeScore:    0.8,
				GenderScore: 0.5,
			},
		},
	}
}