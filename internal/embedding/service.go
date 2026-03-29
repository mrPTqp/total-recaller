package embedding

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

type EmbeddingService struct {
	cfg        *config.Config
	httpClient *http.Client
	tokenMgr   *token.TokenManager
	logger     *zap.Logger
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

func NewEmbeddingService(cfg *config.Config, httpClient *http.Client, tokenManager *token.TokenManager, logger *zap.Logger) *EmbeddingService {
	return &EmbeddingService{
		cfg:        cfg,
		httpClient: httpClient,
		tokenMgr:   tokenManager,
		logger:     logger,
	}
}

func (s *EmbeddingService) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	s.logger.Info("Generating embedding", zap.String("text_preview", text[:min(len(text), 100)]))
	
	request := EmbeddingRequest{
		Model: s.cfg.LLM.EmbeddingModel,
		Input: text,
	}

	token := s.tokenMgr.GetToken(s.cfg.LLM.TokenManager.Scope)
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

	resp, err := s.httpClient.Do(httpReq)
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

	s.logger.Info("Embedding generated successfully", zap.Int("dimensions", len(embedding)))
	return embedding, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}