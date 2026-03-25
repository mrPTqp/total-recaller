package token

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/mrPTqp/total-recaller/internal/config"
)

type TokenManager struct {
	config     *config.Config
	token      string
	httpClient *http.Client
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
}

func NewTokenManager(config *config.Config, httpClient *http.Client) *TokenManager {
	return &TokenManager{
		config:     config,
		httpClient: httpClient,
	}
}

func (tm *TokenManager) RefreshToken(ctx context.Context) error {
	body := "scope=" + tm.config.Transcriber.TokenManager.Scope

	req, err := http.NewRequestWithContext(ctx, "POST", tm.config.Transcriber.TokenManager.URL, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	rqUID := uuid.New().String()
	req.Header.Set("RqUID", rqUID)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+tm.config.TranscriberTokenManagerCredentials)

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

	tm.token = tokenResp.AccessToken

	return nil
}

func (tm *TokenManager) GetToken() string {
	return tm.token
}
