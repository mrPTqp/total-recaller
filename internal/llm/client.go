package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/mrPTqp/total-recaller/internal/config"
	"github.com/mrPTqp/total-recaller/internal/token"
	"go.uber.org/zap"
)

type LLMClient struct {
	cfg        *config.Config
	httpClient *http.Client
	tokenMgr   *token.TokenManager
	logger     *zap.Logger
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatResponse struct {
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Message Message `json:"message"`
}

type EmbeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type EmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func NewGigachatClient(cfg *config.Config, httpClient *http.Client, tokenManager *token.TokenManager, logger *zap.Logger) *LLMClient {
	return &LLMClient{
		cfg:        cfg,
		httpClient: httpClient,
		tokenMgr:   tokenManager,
		logger:     logger,
	}
}

func (c *LLMClient) Summarize(ctx context.Context, text string) (string, error) {
	systemPrompt := `Ты - профессиональный ассистент по суммаризации текстов. 
	Твоя задача - создать краткое, но информативное резюме текста, сохраняя ключевые идеи и важные детали.
	Суммаризируй текст, выделив основные мысли, события и выводы.
	Ответ должен быть на русском языке.`

	request := ChatRequest{
		Model: c.cfg.LLM.GenerateModel,
		Messages: []Message{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: text,
			},
		},
		Stream: false,
	}

	token := c.tokenMgr.GetToken(c.cfg.LLM.TokenManager.Scope)
	if token == "" {
		return "", fmt.Errorf("no access token available")
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://gigachat.devices.sberbank.ru/api/v1/chat/completions", bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices returned from API")
	}

	summary := chatResp.Choices[0].Message.Content
	if summary == "" {
		return "", fmt.Errorf("empty summary returned from API")
	}

	return summary, nil
}

func (c *LLMClient) Chat(ctx context.Context, userMessage string) (string, error) {
	c.logger.Info("Starting chat request", zap.String("user_message", userMessage))
	
	systemPrompt := `Ты - интеллектуальный ассистент. 
	Твоя задача - отвечать на вопросы пользователя, помогать с различными задачами и поддерживать содержательный диалог.
	Отвечай на русском языке, будь вежливым и полезным.`

	c.logger.Info("Creating chat request with model", zap.String("model", c.cfg.LLM.GenerateModel))
	request := ChatRequest{
		Model: c.cfg.LLM.GenerateModel,
		Messages: []Message{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: userMessage,
			},
		},
		Stream: false,
	}

	c.logger.Info("Retrieving access token")
	token := c.tokenMgr.GetToken(c.cfg.LLM.TokenManager.Scope)
	if token == "" {
		c.logger.Error("No access token available")
		return "", fmt.Errorf("no access token available")
	}
	c.logger.Info("Access token retrieved successfully")

	c.logger.Info("Marshaling request to JSON")
	requestBody, err := json.Marshal(request)
	if err != nil {
		c.logger.Error("Failed to marshal request", zap.Error(err))
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}
	c.logger.Info("Request marshaled successfully", zap.Int("request_size", len(requestBody)))

	c.logger.Info("Creating HTTP request")
	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://gigachat.devices.sberbank.ru/api/v1/chat/completions", bytes.NewBuffer(requestBody))
	if err != nil {
		c.logger.Error("Failed to create request", zap.Error(err))
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	c.logger.Info("HTTP request created with headers")

	c.logger.Info("Making HTTP request to Gigachat API")
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.logger.Error("Failed to make request", zap.Error(err))
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()
	c.logger.Info("HTTP request sent, response received")

	c.logger.Info("Reading response body")
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Error("Failed to read response", zap.Error(err))
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	c.logger.Info("Response body read successfully", zap.Int("response_size", len(body)))

	if resp.StatusCode != http.StatusOK {
		c.logger.Error("API returned error status", 
			zap.Int("status_code", resp.StatusCode),
			zap.String("response_body", string(body)))
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}
	c.logger.Info("API response status OK")

	c.logger.Info("Unmarshaling response JSON")
	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		c.logger.Error("Failed to unmarshal response", zap.Error(err))
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}
	c.logger.Info("Response unmarshaled successfully", zap.Int("choices_count", len(chatResp.Choices)))

	if len(chatResp.Choices) == 0 {
		c.logger.Error("No choices returned from API")
		return "", fmt.Errorf("no choices returned from API")
	}

	response := chatResp.Choices[0].Message.Content
	if response == "" {
		c.logger.Error("Empty response returned from API")
		return "", fmt.Errorf("empty response returned from API")
	}

	c.logger.Info("Chat request completed successfully", zap.String("response", response))
	return response, nil
}

func (c *LLMClient) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	c.logger.Info("Generating embedding", zap.String("text_preview", text[:min(len(text), 100)]))
	
	request := EmbeddingRequest{
		Model: c.cfg.LLM.EmbeddingModel,
		Input: text,
	}

	token := c.tokenMgr.GetToken(c.cfg.LLM.TokenManager.Scope)
	if token == "" {
		return nil, fmt.Errorf("no access token available")
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://gigachat.devices.sberbank.ru/api/v1/embeddings", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var embeddingResp EmbeddingResponse
	if err := json.Unmarshal(body, &embeddingResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(embeddingResp.Data) == 0 {
		return nil, fmt.Errorf("no embeddings returned from API")
	}

	embedding := embeddingResp.Data[0].Embedding
	if len(embedding) == 0 {
		return nil, fmt.Errorf("empty embedding returned from API")
	}

	c.logger.Info("Embedding generated successfully", zap.Int("dimensions", len(embedding)))
	return embedding, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
