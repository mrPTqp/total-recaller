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
		Model: c.cfg.LLM.Model,
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
