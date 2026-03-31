package telegrammock

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// handleBotAPI is a catch-all handler for unimplemented endpoints
func (m *TelegramAPIMock) handleBotAPI(w http.ResponseWriter, r *http.Request) {
	m.logger.Debug("Bot API request", zap.String("method", r.Method), zap.String("path", r.URL.Path))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":          true,
		"description": "",
	})
}

func (m *TelegramAPIMock) handleGetUpdates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query()
	offset := 0
	if o := query.Get("offset"); o != "" {
		offset, _ = strconv.Atoi(o)
	}

	timeout := 1
	if t := query.Get("timeout"); t != "" {
		timeout, _ = strconv.Atoi(t)
	}

	allowedUpdates := query["allowed_updates"]
	_ = allowedUpdates

	m.logger.Debug("getUpdates request",
		zap.Int("offset", offset),
		zap.Int("timeout", timeout),
	)

	// Increment polling request counter
	m.stats.mu.Lock()
	m.stats.PollingRequests++
	currentPollingRequests := m.stats.PollingRequests
	m.stats.mu.Unlock()

	m.logger.Debug("Polling request",
		zap.Int("polling_requests", currentPollingRequests),
		zap.Int("offset", offset),
	)

	deadline := time.Now().Add(time.Duration(timeout) * time.Second)
	for {
		m.eventsMu.Lock()
		if len(m.events) > 0 {
			// Get updates that are after the offset and remove them from queue
			var updates []Update
			var remaining []Update
			for _, u := range m.events {
				if int(u.ID) > offset {
					updates = append(updates, u)
				} else {
					remaining = append(remaining, u)
				}
			}

			if len(updates) > 0 {
				// Remove sent updates from queue
				m.events = remaining

				for _, u := range updates {
					m.recordEventType(u)
				}

				m.stats.mu.Lock()
				m.stats.UpdatesSent += len(updates)
				m.stats.mu.Unlock()

				m.eventsMu.Unlock()

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{
					"ok":     true,
					"result": updates,
				})
				return
			}
			m.eventsMu.Unlock()
		} else {
			m.eventsMu.Unlock()
		}

		if time.Now().After(deadline) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"ok":     true,
				"result": []Update{},
			})
			return
		}

		select {
		case <-m.newEvent:
			continue
		case <-time.After(100 * time.Millisecond):
			continue
		}
	}
}

func (m *TelegramAPIMock) handleGetFile(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")

	m.logger.Debug("getFile request received",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.String("content_type", contentType),
	)

	var fileID string

	fileID = r.URL.Query().Get("file_id")

	if fileID == "" && r.Method == http.MethodPost {
		if strings.Contains(contentType, "application/json") {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				m.logger.Error("Failed to parse JSON body", zap.Error(err))
				http.Error(w, "Invalid JSON body", http.StatusBadRequest)
				return
			}
			if id, ok := body["file_id"].(string); ok {
				fileID = id
			}
		} else {
			err := r.ParseForm()
			if err != nil {
				m.logger.Error("Failed to parse form", zap.Error(err))
				http.Error(w, "Invalid form data", http.StatusBadRequest)
				return
			}
			fileID = r.FormValue("file_id")
		}
	}

	if fileID == "" {
		m.logger.Warn("getFile request missing file_id",
			zap.String("method", r.Method),
			zap.String("content_type", contentType),
		)
		http.Error(w, "file_id is required", http.StatusBadRequest)
		return
	}

	m.logger.Debug("Looking up file", zap.String("file_id", fileID))

	m.filesMu.RLock()
	filePath, exists := m.fileStorage[fileID]
	m.filesMu.RUnlock()

	if !exists {
		m.logger.Warn("File not found in storage", zap.String("file_id", fileID))
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		m.logger.Error("Failed to stat file", zap.Error(err))
		http.Error(w, "Failed to stat file", http.StatusInternalServerError)
		return
	}

	filePathForDownload := "voice/" + filepath.Base(filePath)

	m.logger.Debug("Returning file metadata",
		zap.String("file_id", fileID),
		zap.String("file_path", filePathForDownload),
		zap.Int64("file_size", fileInfo.Size()),
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok": true,
		"result": map[string]any{
			"file_id":        fileID,
			"file_unique_id": fileID,
			"file_size":      fileInfo.Size(),
			"file_path":      filePathForDownload,
		},
	})
}

func (m *TelegramAPIMock) handleFileDownload(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/file/")
	if filePath == "" || filePath == r.URL.Path {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}

	m.filesMu.RLock()
	var actualFilePath string
	for _, storedPath := range m.fileStorage {
		if filepath.Base(storedPath) == filepath.Base(filePath) {
			actualFilePath = storedPath
			break
		}
	}
	m.filesMu.RUnlock()

	if actualFilePath == "" {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	data, err := os.ReadFile(actualFilePath)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	m.stats.mu.Lock()
	m.stats.FilesServed++
	m.stats.mu.Unlock()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(actualFilePath)))
	w.Write(data)
}

func (m *TelegramAPIMock) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	// Parse request body to get message content
	var reqBody map[string]any
	if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			m.logger.Error("Failed to parse sendMessage body", zap.Error(err))
		}
	}

	// Extract key information for logging
	chatID := ""
	text := ""
	if reqBody != nil {
		if id, ok := reqBody["chat_id"].(float64); ok {
			chatID = fmt.Sprintf("%.0f", id)
		}
		if t, ok := reqBody["text"].(string); ok {
			text = t
		}
	}

	m.logger.Info("Bot sending message",
		zap.String("chat_id", chatID),
		zap.String("text", text),
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok": true,
		"result": map[string]any{
			"message_id": time.Now().UnixNano(),
			"date":       time.Now().Unix(),
			"chat": map[string]any{
				"id":   0,
				"type": "private",
			},
		},
	})
}

func (m *TelegramAPIMock) handleEditMessageText(w http.ResponseWriter, r *http.Request) {
	// Parse request body to get edited content
	var reqBody map[string]any
	if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			m.logger.Error("Failed to parse editMessageText body", zap.Error(err))
		}
	}

	// Extract key information for logging
	text := ""
	if reqBody != nil {
		if t, ok := reqBody["text"].(string); ok {
			text = t
		}
	}

	m.logger.Info("Bot editing message",
		zap.String("new_text", text),
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok": true,
		"result": map[string]any{
			"message_id": 1,
			"date":       time.Now().Unix(),
		},
	})
}

func (m *TelegramAPIMock) handleSetMyCommands(w http.ResponseWriter, r *http.Request) {
	m.logger.Debug("setMyCommands request received")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":     true,
		"result": true,
	})
}