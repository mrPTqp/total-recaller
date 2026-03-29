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

type GigaChatClient struct {
	cfg        *config.Config
	httpClient *http.Client
	tokenMgr   *token.TokenManager
	logger     *zap.Logger
	semaphore  chan struct{} // Semaphore to limit concurrent requests
}

func NewGigaChatClient(cfg *config.Config, httpClient *http.Client, tokenManager *token.TokenManager, logger *zap.Logger) *GigaChatClient {
	semaphore := make(chan struct{}, 1)
	semaphore <- struct{}{} // Initialize with one token

	return &GigaChatClient{
		cfg:        cfg,
		httpClient: httpClient,
		tokenMgr:   tokenManager,
		logger:     logger,
		semaphore:  semaphore,
	}
}

func (c *GigaChatClient) DoRequest(ctx context.Context, method, url string, body any) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.semaphore:
		defer func() {
			// Release token when done
			c.semaphore <- struct{}{}
		}()
	}

	token := c.tokenMgr.GetToken(c.cfg.LLM.TokenManager.Scope)
	if token == "" {
		return nil, fmt.Errorf("no access token available")
	}

	var requestBody []byte
	var err error
	if body != nil {
		requestBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(requestBody))
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

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(responseBody))
	}

	return responseBody, nil
}
