package llmmock

import "sync"

// MockStats holds statistics for the mock server
type MockStats struct {
	mu                sync.RWMutex
	TokenRequests     int `json:"token_requests"`
	ChatRequests      int `json:"chat_requests"`
	EmbeddingRequests int `json:"embedding_requests"`
}

// TokenResponse represents the OAuth token response
type TokenResponse struct {
	AccessToken string `json:"access_token"`
}