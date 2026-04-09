package token

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/mrPTqp/total-recaller/internal/config"
	"go.uber.org/zap"
)

type TokenManager struct {
	config           *config.Config
	transcriberToken string
	llmToken         string
	mu               sync.RWMutex
	httpClient       *http.Client
	logger           *zap.Logger
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
}

func NewTokenManager(config *config.Config, httpClient *http.Client, logger *zap.Logger) *TokenManager {
	return &TokenManager{
		config:     config,
		httpClient: httpClient,
		logger:     logger,
	}
}

func (tm *TokenManager) RefreshToken(ctx context.Context, scope string) error {
	body := "scope=" + scope

	var url string
	var credentials string

	tm.logger.Info("Refreshing token", zap.String("scope", scope))
	tm.logger.Info("Transcriber scope", zap.String("scope", tm.config.Transcriber.TokenManager.Scope))
	tm.logger.Info("LLM scope", zap.String("scope", tm.config.LLM.TokenManager.Scope))

	switch scope {
	case tm.config.Transcriber.TokenManager.Scope:
		url = tm.config.Transcriber.TokenManager.URL
		credentials = tm.config.TranscriberTokenManagerCredentials
		tm.logger.Info("Using transcriber config", zap.String("url", url))
	case tm.config.LLM.TokenManager.Scope:
		url = tm.config.LLM.TokenManager.URL
		credentials = tm.config.LLMTokenManagerCredentials
		tm.logger.Info("Using LLM config", zap.String("url", url))
	default:
		return fmt.Errorf("unknown scope: %s", scope)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	rqUID := uuid.New().String()
	req.Header.Set("RqUID", rqUID)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+credentials)

	resp, err := tm.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return fmt.Errorf("failed to parse token response: %w", err)
	}

	switch scope {
	case tm.config.Transcriber.TokenManager.Scope:
		tm.mu.Lock()
		tm.transcriberToken = tokenResp.AccessToken
		tm.mu.Unlock()
	case tm.config.LLM.TokenManager.Scope:
		tm.mu.Lock()
		tm.llmToken = tokenResp.AccessToken
		tm.mu.Unlock()
	default:
		tm.logger.Error("unknown scope", zap.String("scope", scope))
	}

	return nil
}

func (tm *TokenManager) GetToken(scope string) string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	switch scope {
	case tm.config.Transcriber.TokenManager.Scope:
		return tm.transcriberToken
	case tm.config.LLM.TokenManager.Scope:
		return tm.llmToken
	default:
		tm.logger.Error("unknown scope", zap.String("scope", scope))
	}
	return ""
}