package llmmock

import (
	"encoding/json"
	"io"
	"net/http"
	"os"

	"github.com/mrPTqp/total-recaller/internal/llm"
	"go.uber.org/zap"
)

func (m *LLMMock) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	m.stats.mu.Lock()
	m.stats.ChatRequests++
	m.stats.mu.Unlock()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get token from environment or use default
	expectedToken := os.Getenv("TOTAL_RECALLER_LLM_TOKEN_MANAGER_CREDENTIALS")
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

	var chatReq llm.ChatRequest
	if err := json.Unmarshal(body, &chatReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	m.messagesMu.Lock()
	for _, msg := range chatReq.Messages {
		if msg.Role == "user" {
			m.lastUserMessage = msg.Content
		}
		if msg.Role == "system" {
			m.lastPrompt = msg.Content
		}
	}
	m.messagesMu.Unlock()

	m.logger.Debug("Chat completion request received",
		zap.String("model", chatReq.Model),
		zap.Int("message_count", len(chatReq.Messages)))

	response := m.generateChatResponse(chatReq)

	// Log the LLM response at INFO level
	var responseText string
	if len(response.Choices) > 0 {
		responseText = response.Choices[0].Message.Content
	}
	m.logger.Info("LLM returning response",
		zap.String("model", chatReq.Model),
		zap.String("response", responseText),
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (m *LLMMock) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	m.stats.mu.Lock()
	m.stats.EmbeddingRequests++
	m.stats.mu.Unlock()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get token from environment or use default
	expectedToken := os.Getenv("TOTAL_RECALLER_LLM_TOKEN_MANAGER_CREDENTIALS")
	if expectedToken == "" {
		expectedToken = "test-credentials"
	}

	if !m.verifyAuth(r, expectedToken) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var embedReq llm.EmbeddingRequest
	if err := json.NewDecoder(r.Body).Decode(&embedReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	m.logger.Debug("Embedding request received",
		zap.String("model", embedReq.Model),
		zap.Int("input_length", len(embedReq.Input)))

	response := m.generateEmbeddingResponse(embedReq)

	// Log the embedding generation at INFO level
	m.logger.Info("LLM generated embeddings",
		zap.String("model", embedReq.Model),
		zap.Int("dimensions", len(response.Data[0].Embedding)),
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (m *LLMMock) verifyAuth(r *http.Request, expectedToken string) bool {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false
	}

	return authHeader == "Bearer " + expectedToken
}

func (m *LLMMock) generateChatResponse(req llm.ChatRequest) llm.ChatResponse {
	var userMessage string
	for _, msg := range req.Messages {
		if msg.Role == "user" {
			userMessage = msg.Content
			break
		}
	}

	responseText := m.generateResponseText(userMessage)

	return llm.ChatResponse{
		Choices: []llm.Choice{
			{
				Message: llm.Message{
					Role:    "assistant",
					Content: responseText,
				},
			},
		},
	}
}

func (m *LLMMock) generateResponseText(userMessage string) string {
	switch {
	case containsKeyword(userMessage, "привет", "здравствуй", "hello"):
		return "Здравствуйте! Чем я могу вам помочь?"
	case containsKeyword(userMessage, "пока", "до свидания", "goodbye"):
		return "До свидания! Обращайтесь ещё!"
	case containsKeyword(userMessage, "спасибо", "благодарю"):
		return "Всегда пожалуйста! Рад помочь!"
	case containsKeyword(userMessage, "суммариз", "кратк", "резюм"):
		return "Это краткое содержание предоставленного текста с выделением ключевых моментов и основных идей."
	default:
		return "Я понял ваш запрос. Вот мой ответ на поставленный вопрос."
	}
}

func containsKeyword(text string, keywords ...string) bool {
	for _, keyword := range keywords {
		if containsIgnoreCase(text, keyword) {
			return true
		}
	}
	return false
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s[:len(substr)] == substr || containsLower(s, substr))
}

func containsLower(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if matchIgnoreCase(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func matchIgnoreCase(s, substr string) bool {
	if len(s) != len(substr) {
		return false
	}
	for i := 0; i < len(s); i++ {
		if toLower(s[i]) != toLower(substr[i]) {
			return false
		}
	}
	return true
}

func toLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}

func (m *LLMMock) generateEmbeddingResponse(req llm.EmbeddingRequest) llm.EmbeddingResponse {
	dimensions := 768
	embedding := make([]float32, dimensions)

	hash := simpleHash(req.Input)
	for i := 0; i < dimensions; i++ {
		embedding[i] = float32(((hash+i*7)%1000)-500) / 500.0
	}

	return llm.EmbeddingResponse{
		Data: []struct {
			Embedding []float32 `json:"embedding"`
		}{
			{
				Embedding: embedding,
			},
		},
	}
}

func simpleHash(s string) int {
	hash := 0
	for i := 0; i < len(s); i++ {
		hash = ((hash << 5) - hash) + int(s[i])
		hash |= 0
	}
	return hash
}