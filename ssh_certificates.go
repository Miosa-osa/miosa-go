package miosa

import (
	"context"
	"errors"
	"net/url"
)

// MaxSSHCertificateTTLSeconds is the longest certificate lifetime the API signs.
const MaxSSHCertificateTTLSeconds = 3600

// SSHCertificateInput asks for a short-lived SSH certificate for one sandbox.
type SSHCertificateInput struct {
	// PublicKey is the OpenSSH ed25519, ecdsa or rsa public key to sign.
	PublicKey string `json:"public_key"`
	// TTLSeconds is the lifetime, 1 to 3600; zero uses the API default
	// (15 minutes).
	TTLSeconds int `json:"ttl_seconds,omitempty"`
}

// SSHCertificate is a signed, machine-bound SSH certificate. Connect with the
// private key and this certificate through the authenticated tunnel at
// TunnelURL; nothing stays authorized in the sandbox afterwards.
type SSHCertificate struct {
	Certificate string `json:"certificate"`
	// Principal is "sandbox-<id>"; another sandbox's sshd refuses it.
	Principal   string `json:"principal"`
	User        string `json:"user"`
	KeyID       string `json:"key_id"`
	Serial      int64  `json:"serial"`
	ValidAfter  string `json:"valid_after"`
	ValidBefore string `json:"valid_before"`
	TTLSeconds  int    `json:"ttl_seconds"`
	CAPublicKey string `json:"ca_public_key"`
	SandboxID   string `json:"sandbox_id"`
	Transport   string `json:"transport"`
	TunnelURL   string `json:"tunnel_url"`
}

// IssueSSHCertificate signs a short-lived SSH certificate for a running
// sandbox (POST /sandboxes/:id/ssh-certificates, scope sandboxes:exec). The
// sandbox must be running (409 SANDBOX_NOT_RUNNING otherwise).
func (s *SandboxesService) IssueSSHCertificate(ctx context.Context, id string, in SSHCertificateInput) (*SSHCertificate, error) {
	if in.PublicKey == "" {
		return nil, errors.New("public_key is required")
	}
	if in.TTLSeconds < 0 || in.TTLSeconds > MaxSSHCertificateTTLSeconds {
		return nil, errors.New("ttl_seconds must be between 1 and 3600")
	}
	var out apiResponse[SSHCertificate]
	if err := s.client.postJSON(ctx, "/sandboxes/"+url.PathEscape(id)+"/ssh-certificates", in, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// SSHCertificateInfo says how to get a short-lived certificate for a sandbox.
type SSHCertificateInfo struct {
	// Available is false until the control plane has an SSH CA configured.
	Available bool   `json:"available"`
	Method    string `json:"method"`
	Endpoint  string `json:"endpoint"`
	Principal string `json:"principal"`
	// PrincipalFormat is "sandbox-<sandbox id>".
	PrincipalFormat   string `json:"principal_format"`
	DefaultTTLSeconds int    `json:"default_ttl_seconds"`
	MaxTTLSeconds     int    `json:"max_ttl_seconds"`
}

// SSHInfo is how to reach a running sandbox's sshd: through the authenticated
// WebSocket tunnel at TunnelURL.
type SSHInfo struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	User      string `json:"user"`
	GuestIP   string `json:"guest_ip"`
	Transport string `json:"transport"`
	TunnelURL string `json:"tunnel_url"`
	// ProxyJump and Command are nil: the tunnel replaced the jump host.
	ProxyJump    *string            `json:"proxy_jump"`
	Command      *string            `json:"command"`
	Certificates SSHCertificateInfo `json:"certificates"`
}

// SSHInfo returns the SSH connection details of a running sandbox
// (GET /sandboxes/:id/ssh-info, scope sandboxes:read).
func (s *SandboxesService) SSHInfo(ctx context.Context, id string) (*SSHInfo, error) {
	var out SSHInfo
	if err := s.client.getJSON(ctx, "/sandboxes/"+url.PathEscape(id)+"/ssh-info", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// InstallSSHKeyInput is the body of installing a key into a sandbox.
type InstallSSHKeyInput struct {
	// PublicKey is an OpenSSH public key (ssh-ed25519, ssh-rsa, ecdsa-sha2-*, sk-*).
	PublicKey string `json:"public_key"`
	// Certificate asks for a short-lived certificate instead of a key that
	// stays authorized; the call then returns the certificate.
	Certificate bool `json:"certificate,omitempty"`
	// TTLSeconds applies to Certificate only.
	TTLSeconds int `json:"ttl_seconds,omitempty"`
}

// InstallSSHKey installs a public key in a running sandbox's authorized keys
// (POST /sandboxes/:id/ssh-keys, scope sandboxes:exec). It returns nil on the
// plain form (204). With Certificate set it returns the signed certificate,
// the same as IssueSSHCertificate.
func (s *SandboxesService) InstallSSHKey(ctx context.Context, id string, in InstallSSHKeyInput) (*SSHCertificate, error) {
	if in.PublicKey == "" {
		return nil, errors.New("public_key is required")
	}
	path := "/sandboxes/" + url.PathEscape(id) + "/ssh-keys"
	if !in.Certificate {
		return nil, s.client.postJSON(ctx, path, in, nil)
	}
	var out apiResponse[SSHCertificate]
	if err := s.client.postJSON(ctx, path, in, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}
