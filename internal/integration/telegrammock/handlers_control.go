package telegrammock

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"

	"go.uber.org/zap"
)

func (m *TelegramAPIMock) handleInjectText(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, _ := strconv.ParseInt(r.URL.Query().Get("user_id"), 10, 64)
	text := r.URL.Query().Get("text")

	if userID == 0 || text == "" {
		http.Error(w, "user_id and text are required", http.StatusBadRequest)
		return
	}

	update := m.createTextMessageUpdate(userID, text)
	m.queueUpdate(update)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":        true,
		"update_id": update.ID,
	})
}

func (m *TelegramAPIMock) handleInjectVoice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, _ := strconv.ParseInt(r.URL.Query().Get("user_id"), 10, 64)
	filePath := r.URL.Query().Get("file_path")

	if userID == 0 || filePath == "" {
		http.Error(w, "user_id and file_path are required", http.StatusBadRequest)
		return
	}

	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusBadRequest)
		return
	}

	update, err := m.createVoiceMessageUpdate(userID, filePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	m.queueUpdate(update)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":        true,
		"update_id": update.ID,
	})
}

func (m *TelegramAPIMock) handleInjectAudio(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, _ := strconv.ParseInt(r.URL.Query().Get("user_id"), 10, 64)
	filePath := r.URL.Query().Get("file_path")

	if userID == 0 || filePath == "" {
		http.Error(w, "user_id and file_path are required", http.StatusBadRequest)
		return
	}

	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusBadRequest)
		return
	}

	update, err := m.createAudioMessageUpdate(userID, filePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	m.queueUpdate(update)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":        true,
		"update_id": update.ID,
	})
}

func (m *TelegramAPIMock) handleInjectCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, _ := strconv.ParseInt(r.URL.Query().Get("user_id"), 10, 64)
	command := r.URL.Query().Get("command")

	if userID == 0 || command == "" {
		http.Error(w, "user_id and command are required", http.StatusBadRequest)
		return
	}

	update := m.createCommandUpdate(userID, command)
	m.queueUpdate(update)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":        true,
		"update_id": update.ID,
	})
}

func (m *TelegramAPIMock) handleGetStats(w http.ResponseWriter, r *http.Request) {
	m.stats.mu.RLock()
	defer m.stats.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok": true,
		"result": map[string]any{
			"updates_sent":   m.stats.UpdatesSent,
			"files_served":   m.stats.FilesServed,
			"events_by_type": m.stats.EventsByType,
		},
	})
}

func (m *TelegramAPIMock) handleResetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	m.stats.mu.Lock()
	m.stats.UpdatesSent = 0
	m.stats.FilesServed = 0
	m.stats.EventsByType = make(map[string]int)
	m.stats.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok": true,
	})
}

func (m *TelegramAPIMock) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":     true,
		"status": "healthy",
	})
}

// LogHandlerRequest logs an incoming request - useful for debugging.
func (m *TelegramAPIMock) LogHandlerRequest(handlerName string, r *http.Request) {
	m.logger.Debug(handlerName+" request received",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
	)
}