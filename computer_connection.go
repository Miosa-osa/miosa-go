package miosa

import (
	"context"
	"errors"
	"net/url"
	"strconv"
)

// StreamAccess is what stream-token, embed and viewer-session return. Fields
// that a route does not send stay zero.
type StreamAccess struct {
	ComputerID      string `json:"computer_id,omitempty"`
	Slug            string `json:"slug,omitempty"`
	Token           string `json:"token"`
	ExpiresAt       int64  `json:"expires_at"`
	TTLSeconds      int    `json:"ttl_seconds"`
	EmbedURL        string `json:"embed_url,omitempty"`
	DesktopURL      string `json:"desktop_url,omitempty"`
	DesktopEntryURL string `json:"desktop_entry_url,omitempty"`
	WSURL           string `json:"ws_url,omitempty"`
}

// ViewerPasswordStatus says whether a viewer password is set.
type ViewerPasswordStatus struct {
	ComputerID          string `json:"computer_id"`
	ViewerPasswordSet   bool   `json:"viewer_password_set"`
	ViewerPasswordSetAt string `json:"viewer_password_set_at,omitempty"`
}

// ViewerPassword is the new plaintext password; it is shown once.
type ViewerPassword struct {
	ComputerID          string `json:"computer_id"`
	ViewerPassword      string `json:"viewer_password"`
	ViewerPasswordSetAt string `json:"viewer_password_set_at"`
}

func ttlBody(ttlSeconds int) any {
	if ttlSeconds <= 0 {
		return nil
	}
	return map[string]any{"ttl_seconds": ttlSeconds}
}

// StreamToken mints a short-lived token for the desktop stream. ttlSeconds 0
// uses the server default; a value out of range is a 422 INVALID_TTL.
func (s *ComputersService) StreamToken(ctx context.Context, id string, ttlSeconds int) (*StreamAccess, error) {
	var out StreamAccess
	if err := s.client.postJSON(ctx, "/computers/"+url.PathEscape(id)+"/stream-token", ttlBody(ttlSeconds), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Embed returns URLs for an iframe or WebSocket viewer. The computer must be
// running (409 computer_not_running otherwise).
func (s *ComputersService) Embed(ctx context.Context, id string, ttlSeconds int) (*StreamAccess, error) {
	q := map[string]string{}
	if ttlSeconds > 0 {
		q["ttl_seconds"] = strconv.Itoa(ttlSeconds)
	}
	var out StreamAccess
	if err := s.client.getJSON(ctx, "/computers/"+url.PathEscape(id)+"/embed"+buildQuery(q), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// VNCCredentials returns the raw credentials document for the desktop.
func (s *ComputersService) VNCCredentials(ctx context.Context, id string) (map[string]any, error) {
	var out map[string]any
	if err := s.client.getJSON(ctx, "/computers/"+url.PathEscape(id)+"/vnc-credentials", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ViewerPasswordStatus reports whether a viewer password is set.
func (s *ComputersService) ViewerPasswordStatus(ctx context.Context, id string) (*ViewerPasswordStatus, error) {
	var out ViewerPasswordStatus
	if err := s.client.getJSON(ctx, "/computers/"+url.PathEscape(id)+"/viewer-password", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RotateViewerPassword sets a new viewer password and returns it once.
func (s *ComputersService) RotateViewerPassword(ctx context.Context, id string) (*ViewerPassword, error) {
	var out ViewerPassword
	if err := s.client.postJSON(ctx, "/computers/"+url.PathEscape(id)+"/viewer-password/rotate", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateViewerSession trades a viewer password for a stream token. It needs no
// account credential, so end users can call it. A wrong password is 401
// wrong_password; too many tries is 429 rate_limited.
func (s *ComputersService) CreateViewerSession(ctx context.Context, id, password string, ttlSeconds int) (*StreamAccess, error) {
	if password == "" {
		return nil, errors.New("miosa: viewer password is required")
	}
	body := map[string]any{"password": password}
	if ttlSeconds > 0 {
		body["ttl_seconds"] = ttlSeconds
	}
	var out StreamAccess
	if err := s.client.postJSON(ctx, "/computers/"+url.PathEscape(id)+"/viewer-session", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
