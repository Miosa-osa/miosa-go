package miosa

import (
	"context"
	"time"
)

// WhoamiUser is the signed-in person behind the credential, when there is one.
type WhoamiUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// WhoamiScopeRef names an organization or workspace.
type WhoamiScopeRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// WhoamiKey summarizes the API key in use. The key itself is never returned.
type WhoamiKey struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Prefix    string     `json:"prefix"`
	Type      string     `json:"type"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// WhoamiAuth describes how the request authenticated.
type WhoamiAuth struct {
	// Method is "api_key", "session" and so on.
	Method string   `json:"method"`
	Scopes []string `json:"scopes"`
	// Unrestricted is true for admin and platform keys and owner/admin roles,
	// which bypass resource scopes.
	Unrestricted bool `json:"unrestricted"`
	// Key is nil for a session.
	Key *WhoamiKey `json:"key"`
}

// Whoami is who the presented credential is: user, organization, workspace,
// plan and scopes, in one call.
type Whoami struct {
	// User is nil for a key that is not tied to a person.
	User *WhoamiUser `json:"user"`
	// Organization is nil when the credential resolves to none.
	Organization *WhoamiScopeRef `json:"organization"`
	// Workspace is nil unless the credential is bound to a workspace.
	Workspace *WhoamiScopeRef `json:"workspace"`
	Plan      *struct {
		Name string `json:"name"`
	} `json:"plan"`
	Auth WhoamiAuth `json:"auth"`
}

// Whoami returns who the credential in use is (GET /whoami). It needs no
// scope and works for API keys and browser sessions.
func (c *Client) Whoami(ctx context.Context) (*Whoami, error) {
	var out Whoami
	if err := c.getJSON(ctx, "/whoami", &out); err != nil {
		return nil, err
	}
	return &out, nil
}
