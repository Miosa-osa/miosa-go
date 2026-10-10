package miosa

import (
	"context"
	"errors"
	"net/url"
	"time"
)

// Scoped delegation tokens are how a white-label platform lets one of its own
// end users call MIOSA directly: the platform authenticates with its master
// API key, mints a short-lived token bound to one user and one workspace with a
// subset of its own scopes, and hands that token to the user's client.
//
// A token can never carry more than the minting credential holds, cannot
// itself mint tokens, and is limited to its workspace. Use Client.AsUser to
// build a client that authenticates as the token.

// MaxScopedTokenTTLSeconds is the longest lifetime a scoped token can have.
const MaxScopedTokenTTLSeconds = 86400

// DefaultScopedTokenTTLSeconds is the lifetime the API applies when none is given.
const DefaultScopedTokenTTLSeconds = 3600

// MintScopedTokenInput describes the token to mint.
type MintScopedTokenInput struct {
	// UserID is your end user's identifier. It becomes the token's subject.
	UserID string `json:"user_id"`
	// WorkspaceID is the workspace the token is limited to. It must belong to
	// the minting organization.
	WorkspaceID string `json:"workspace_id"`
	// Scopes is the subset of the minting credential's scopes the token gets.
	// Asking for more is refused (403), an unknown scope is 422.
	Scopes []string `json:"scopes,omitempty"`
	// ExpiresInSeconds is the lifetime, 1 to 86400. Zero uses the default (one hour).
	ExpiresInSeconds int `json:"expires_in_seconds,omitempty"`
}

// ScopedToken is a freshly minted token. Token is shown once and cannot be
// retrieved again; the id lets you list and revoke it.
type ScopedToken struct {
	ID        string   `json:"id"`
	Token     string   `json:"token"`
	ExpiresAt string   `json:"expires_at"`
	Scopes    []string `json:"scopes"`
}

// ScopedTokenInfo is a minted token as listed. The token itself is never returned.
type ScopedTokenInfo struct {
	ID          string   `json:"id"`
	UserID      string   `json:"user_id"`
	WorkspaceID string   `json:"workspace_id"`
	Scopes      []string `json:"scopes"`
	ExpiresAt   string   `json:"expires_at"`
	RevokedAt   string   `json:"revoked_at,omitempty"`
	CreatedAt   string   `json:"created_at"`
}

// Active reports whether the token is neither revoked nor expired at the given
// time.
func (t ScopedTokenInfo) Active(at time.Time) bool {
	if t.RevokedAt != "" {
		return false
	}
	exp, err := time.Parse(time.RFC3339Nano, t.ExpiresAt)
	return err != nil || at.Before(exp)
}

// ScopedTokenPage is one page of minted tokens, newest first.
type ScopedTokenPage struct {
	Data []ScopedTokenInfo `json:"data"`
	// NextCursor is the id to pass as Before for the next page; empty on the last.
	NextCursor string `json:"next_cursor"`
}

// ScopedTokensService mints, lists and revokes scoped end-user tokens.
// Accessed via Client.ScopedTokens. A scoped token or sandbox token cannot call it.
type ScopedTokensService struct {
	client *Client
}

// Mint creates a scoped token (POST /tokens/scoped).
func (s *ScopedTokensService) Mint(ctx context.Context, in MintScopedTokenInput) (*ScopedToken, error) {
	if in.UserID == "" {
		return nil, errors.New("user_id is required")
	}
	if in.WorkspaceID == "" {
		return nil, errors.New("workspace_id is required")
	}
	if in.ExpiresInSeconds < 0 || in.ExpiresInSeconds > MaxScopedTokenTTLSeconds {
		return nil, errors.New("expires_in_seconds must be between 1 and 86400")
	}
	var out ScopedToken
	if err := s.client.postJSON(ctx, "/tokens/scoped", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// List returns one page of minted tokens, 50 at a time. before is the previous
// page's NextCursor; empty starts at the newest.
func (s *ScopedTokensService) List(ctx context.Context, before string) (*ScopedTokenPage, error) {
	var out ScopedTokenPage
	if err := s.client.getJSON(ctx, "/tokens/scoped"+buildQuery(map[string]string{"before": before}), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// All walks every page and returns all minted tokens.
func (s *ScopedTokensService) All(ctx context.Context) ([]ScopedTokenInfo, error) {
	var all []ScopedTokenInfo
	cursor := ""
	for {
		page, err := s.List(ctx, cursor)
		if err != nil {
			return nil, err
		}
		all = append(all, page.Data...)
		if page.NextCursor == "" {
			return all, nil
		}
		cursor = page.NextCursor
	}
}

// Revoke revokes a token by id (DELETE /tokens/scoped/:id). 404 for an unknown id.
func (s *ScopedTokensService) Revoke(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("id is required")
	}
	var out struct {
		Revoked bool `json:"revoked"`
	}
	return s.client.deleteJSON(ctx, "/tokens/scoped/"+url.PathEscape(id), &out)
}
