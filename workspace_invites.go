package miosa

import (
	"context"
	"fmt"
)

// WorkspaceInvitesService manages the email invite flow for workspace access.
type WorkspaceInvitesService struct {
	client *Client
}

func (s *WorkspaceInvitesService) base(workspaceID string) string {
	return fmt.Sprintf("/workspaces/%s/invites", workspaceID)
}

// Create sends a workspace invite to email or adds the user directly if they
// are already a tenant member.
//
// When the email already maps to a tenant member the user is added directly and
// CreateWorkspaceInviteResponse.Type == "added". Otherwise an invite row is
// created and Type == "invited".
//
// POST /workspaces/:id/invites
func (s *WorkspaceInvitesService) Create(ctx context.Context, workspaceID string, input CreateWorkspaceInviteInput) (*CreateWorkspaceInviteResponse, error) {
	const op = "WorkspaceInvitesService.Create"
	var out CreateWorkspaceInviteResponse
	if err := s.client.postJSON(ctx, s.base(workspaceID), input, &out); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &out, nil
}

// List returns all pending (non-expired, non-accepted, non-revoked) invites for
// the workspace.
//
// GET /workspaces/:id/invites
func (s *WorkspaceInvitesService) List(ctx context.Context, workspaceID string) (*WorkspaceInviteListResponse, error) {
	const op = "WorkspaceInvitesService.List"
	var out WorkspaceInviteListResponse
	if err := s.client.getJSON(ctx, s.base(workspaceID), &out); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &out, nil
}

// Revoke revokes a pending invite. Already-revoked invites are idempotent.
// An invite that was legitimately accepted returns a 409 error.
//
// DELETE /workspaces/:id/invites/:invite_id
func (s *WorkspaceInvitesService) Revoke(ctx context.Context, workspaceID, inviteID string) (*WorkspaceInviteRevokeResponse, error) {
	const op = "WorkspaceInvitesService.Revoke"
	var out WorkspaceInviteRevokeResponse
	url := fmt.Sprintf("%s/%s", s.base(workspaceID), inviteID)
	if err := s.client.deleteJSON(ctx, url, &out); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &out, nil
}

// Preview returns a public preview of the invite without authentication.
// Returns nil when the token is unknown or has been revoked.
//
// GET /workspace-invites/:token
func (s *WorkspaceInvitesService) Preview(ctx context.Context, token string) (*WorkspaceInvitePreview, error) {
	const op = "WorkspaceInvitesService.Preview"
	var out WorkspaceInvitePreviewResponse
	url := fmt.Sprintf("/workspace-invites/%s", token)
	if err := s.client.getJSON(ctx, url, &out); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &out.Data, nil
}

// Accept claims the workspace invite on behalf of the authenticated user.
//
// The caller's API key / JWT email must match the invite email.
//
// POST /workspace-invites/:token/accept
func (s *WorkspaceInvitesService) Accept(ctx context.Context, token string) (*AcceptWorkspaceInviteResponse, error) {
	const op = "WorkspaceInvitesService.Accept"
	var out AcceptWorkspaceInviteResponse
	url := fmt.Sprintf("/workspace-invites/%s/accept", token)
	if err := s.client.postJSON(ctx, url, struct{}{}, &out); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &out, nil
}
