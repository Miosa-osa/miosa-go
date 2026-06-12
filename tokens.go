package miosa

import (
	"context"
	"fmt"
)

// TokensService mints short-lived user/workspace scoped tokens.
type TokensService struct {
	client *Client
}

// CreateScopedTokenInput is the request body for POST /tokens/scoped.
type CreateScopedTokenInput struct {
	UserID           string   `json:"user_id"`
	WorkspaceID      string   `json:"workspace_id"`
	ExpiresInSeconds int      `json:"expires_in_seconds,omitempty"`
	Scopes           []string `json:"scopes,omitempty"`
}

// ScopedTokenResult is returned by Tokens.CreateScoped.
type ScopedTokenResult struct {
	Token     string   `json:"token"`
	ExpiresAt string   `json:"expires_at,omitempty"`
	Scopes    []string `json:"scopes,omitempty"`
}

// CreateScoped mints a short-lived token for one MIOSA user/workspace pair.
func (s *TokensService) CreateScoped(ctx context.Context, input CreateScopedTokenInput) (*ScopedTokenResult, error) {
	var env apiResponse[ScopedTokenResult]
	if err := s.client.postJSON(ctx, "/tokens/scoped", input, &env); err != nil {
		return nil, fmt.Errorf("TokensService.CreateScoped: %w", err)
	}
	return &env.Data, nil
}
