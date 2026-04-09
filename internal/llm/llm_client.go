package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mrPTqp/total-recaller/internal/config"
	"github.com/mrPTqp/total-recaller/internal/token"
	"go.uber.org/zap"
)

type LLMClient struct {
	cfg        *config.Config
	httpClient *GigaChatClient
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

func NewLLMClient(cfg *config.Config, httpClient *http.Client, tokenManager *token.TokenManager, logger *zap.Logger) *LLMClient {
	return &LLMClient{
		cfg:        cfg,
		httpClient: NewGigaChatClient(cfg, httpClient, tokenManager, logger),
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

	responseBody, err := c.httpClient.DoRequest(ctx, "POST", "https://gigachat.devices.sberbank.ru/api/v1/chat/completions", request)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(responseBody, &chatResp); err != nil {
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

	responseBody, err := c.httpClient.DoRequest(ctx, "POST", "https://gigachat.devices.sberbank.ru/api/v1/chat/completions", request)
	if err != nil {
		c.logger.Error("Failed to make request", zap.Error(err))
		return "", fmt.Errorf("failed to make request: %w", err)
	}

	c.logger.Info("HTTP request sent, response received")

	c.logger.Info("Unmarshaling response JSON")
	var chatResp ChatResponse
	if err := json.Unmarshal(responseBody, &chatResp); err != nil {
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

	responseBody, err := c.httpClient.DoRequest(ctx, "POST", "https://gigachat.devices.sberbank.ru/api/v1/embeddings", request)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	var embeddingResp EmbeddingResponse
	if err := json.Unmarshal(responseBody, &embeddingResp); err != nil {
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
