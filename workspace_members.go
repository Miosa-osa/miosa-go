package miosa

import (
	"context"
	"fmt"
)

// WorkspaceMembersService manages the per-workspace user roster.
type WorkspaceMembersService struct {
	client *Client
}

func (s *WorkspaceMembersService) base(workspaceID string) string {
	return fmt.Sprintf("/workspaces/%s/members", workspaceID)
}

// ListWorkspaceMembers returns all members of a workspace.
//
// GET /workspaces/:id/members
func (s *WorkspaceMembersService) List(ctx context.Context, workspaceID string) (*WorkspaceMemberListResponse, error) {
	const op = "WorkspaceMembersService.List"
	var out WorkspaceMemberListResponse
	if err := s.client.getJSON(ctx, s.base(workspaceID), &out); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &out, nil
}

// Add adds an existing tenant user to the workspace.
//
// The user must already hold a tenant_members row for the parent org. To
// invite someone who is not yet an org member use WorkspaceInvitesService.Create.
//
// POST /workspaces/:id/members
func (s *WorkspaceMembersService) Add(ctx context.Context, workspaceID string, input AddWorkspaceMemberInput) (*WorkspaceMemberRecordResponse, error) {
	const op = "WorkspaceMembersService.Add"
	var out WorkspaceMemberRecordResponse
	if err := s.client.postJSON(ctx, s.base(workspaceID), input, &out); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &out, nil
}

// UpdateRole changes a workspace member's role.
//
// PATCH /workspaces/:id/members/:user_id
func (s *WorkspaceMembersService) UpdateRole(ctx context.Context, workspaceID, userID string, input UpdateWorkspaceMemberRoleInput) (*WorkspaceMemberRecordResponse, error) {
	const op = "WorkspaceMembersService.UpdateRole"
	var out WorkspaceMemberRecordResponse
	url := fmt.Sprintf("%s/%s", s.base(workspaceID), userID)
	if err := s.client.patchJSON(ctx, url, input, &out); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &out, nil
}

// Remove removes a user from the workspace.
//
// The last owner cannot be removed; promote another member to owner first.
//
// DELETE /workspaces/:id/members/:user_id
func (s *WorkspaceMembersService) Remove(ctx context.Context, workspaceID, userID string) (*WorkspaceMemberDeleteResponse, error) {
	const op = "WorkspaceMembersService.Remove"
	var out WorkspaceMemberDeleteResponse
	url := fmt.Sprintf("%s/%s", s.base(workspaceID), userID)
	if err := s.client.deleteJSON(ctx, url, &out); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &out, nil
}
