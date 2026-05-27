package miosa

import (
	"context"
	"fmt"
)

// OrgInvitesService manages the email invite flow for org (tenant) membership.
type OrgInvitesService struct {
	client *Client
}

func (s *OrgInvitesService) base(tenantID string) string {
	return fmt.Sprintf("/tenants/%s/invites", tenantID)
}

// Create creates an org invite and dispatches the invite email.
//
// The response includes an InviteURL that is host-aware: on white-label tenants
// it uses the tenant's custom domain. Requires admin or owner role in the tenant.
//
// POST /tenants/:id/invites
func (s *OrgInvitesService) Create(ctx context.Context, tenantID string, input CreateOrgInviteInput) (*OrgInviteCreated, error) {
	const op = "OrgInvitesService.Create"
	var out OrgInviteCreatedResponse
	if err := s.client.postJSON(ctx, s.base(tenantID), input, &out); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &out.Data, nil
}

// List returns all pending (non-expired, non-accepted, non-revoked) org invites.
//
// Requires admin or owner role in the tenant.
//
// GET /tenants/:id/invites
func (s *OrgInvitesService) List(ctx context.Context, tenantID string) (*OrgInviteListResponse, error) {
	const op = "OrgInvitesService.List"
	var out OrgInviteListResponse
	if err := s.client.getJSON(ctx, s.base(tenantID), &out); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &out, nil
}

// Revoke revokes a pending org invite. Returns 409 when the invite was already
// legitimately accepted.
//
// Requires admin or owner role in the tenant.
//
// DELETE /tenants/:id/invites/:invite_id
func (s *OrgInvitesService) Revoke(ctx context.Context, tenantID, inviteID string) (*OrgInviteRevokeResponse, error) {
	const op = "OrgInvitesService.Revoke"
	var out OrgInviteRevokeResponse
	url := fmt.Sprintf("%s/%s", s.base(tenantID), inviteID)
	if err := s.client.deleteJSON(ctx, url, &out); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &out, nil
}

// Preview returns a public preview of the org invite without authentication.
// Returns nil when the token is unknown or has been revoked.
//
// GET /invites/:token
func (s *OrgInvitesService) Preview(ctx context.Context, token string) (*OrgInvitePreview, error) {
	const op = "OrgInvitesService.Preview"
	var out OrgInvitePreviewResponse
	url := fmt.Sprintf("/invites/%s", token)
	if err := s.client.getJSON(ctx, url, &out); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &out.Data, nil
}

// Accept claims the org invite on behalf of the authenticated user.
//
// The caller's API key / JWT email must match the invite email.
// Error 400 means invalid or expired token; 422 means email mismatch.
//
// POST /invites/:token/accept
func (s *OrgInvitesService) Accept(ctx context.Context, token string) (*AcceptOrgInviteResponse, error) {
	const op = "OrgInvitesService.Accept"
	var out AcceptOrgInviteResponse
	url := fmt.Sprintf("/invites/%s/accept", token)
	if err := s.client.postJSON(ctx, url, struct{}{}, &out); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &out, nil
}
