// Package sandboxssh connects to a MIOSA sandbox over SSH from Go.
//
// A sandbox's sshd is reached through an authenticated WebSocket tunnel, not a
// public port. Dial hides the steps: generate a key (or bring one), ask the API
// for a short-lived certificate bound to the sandbox, open the tunnel, and run
// the SSH handshake over it. The result is an ordinary *ssh.Client from
// golang.org/x/crypto/ssh, so sessions, port forwards and SFTP work as usual.
//
//	client := miosa.NewClient("msk_u_...")
//	conn, err := sandboxssh.Dial(ctx, client, sandboxID, sandboxssh.Options{})
//	if err != nil { ... }
//	defer conn.Close()
//	session, _ := conn.NewSession()
//	out, _ := session.Output("uname -a")
//
// Nothing stays authorized in the sandbox: the certificate expires on its own
// (15 minutes by default).
package sandboxssh

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"

	miosa "github.com/Miosa-osa/miosa-go/v2"
)

// KeyPair is an SSH key pair.
type KeyPair struct {
	// Signer signs for the private half.
	Signer ssh.Signer
	// PrivateKeyPEM is the private key in OpenSSH PEM form, ready to write to a
	// file with mode 0600.
	PrivateKeyPEM []byte
	// PublicKey is the authorized_keys line, without a trailing newline.
	PublicKey string
}

// GenerateKey creates an Ed25519 key pair. comment is appended to the public
// key line and may be empty.
func GenerateKey(comment string) (*KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("sandboxssh: generate key: %w", err)
	}
	block, err := ssh.MarshalPrivateKey(priv, comment)
	if err != nil {
		return nil, fmt.Errorf("sandboxssh: marshal private key: %w", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		return nil, fmt.Errorf("sandboxssh: signer: %w", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil, fmt.Errorf("sandboxssh: public key: %w", err)
	}
	line := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub)))
	if comment != "" {
		line += " " + comment
	}
	return &KeyPair{Signer: signer, PrivateKeyPEM: pem.EncodeToMemory(block), PublicKey: line}, nil
}

// ParsePrivateKey loads a key pair from an OpenSSH or PKCS#8 PEM private key.
func ParsePrivateKey(pemBytes []byte) (*KeyPair, error) {
	signer, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		return nil, fmt.Errorf("sandboxssh: parse private key: %w", err)
	}
	return &KeyPair{
		Signer:        signer,
		PrivateKeyPEM: pemBytes,
		PublicKey:     strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey()))),
	}, nil
}

// Options tune Dial. The zero value works.
type Options struct {
	// Key is the key to certify. A fresh Ed25519 key is generated per Dial when nil.
	Key *KeyPair
	// TTLSeconds is the certificate lifetime (1 to 3600); zero uses the API default.
	TTLSeconds int
	// User overrides the login user; the default is the one the certificate names (root).
	User string
	// HostKeyCallback verifies the guest's host key. When nil the host key is
	// not checked: the tunnel is authenticated and TLS-protected up to MIOSA,
	// which resolves the sandbox by id, so there is no path for an
	// impersonator, but pass a callback if you pin host keys.
	HostKeyCallback ssh.HostKeyCallback
	// Timeout bounds the tunnel dial and the SSH handshake. Default 30 s.
	Timeout time.Duration
	// ClientVersion sets the SSH identification string without the "SSH-2.0-"
	// prefix, for white-label products. Default "miosa-go".
	ClientVersion string
}

// Result is a connected session with the material that made it.
type Result struct {
	// Client is the connected SSH client.
	Client *ssh.Client
	// Key is the key pair used; keep PrivateKeyPEM to reconnect with the same
	// identity while the certificate lives.
	Key *KeyPair
	// Certificate is what the API issued.
	Certificate *miosa.SSHCertificate
}

// Dial connects to a running sandbox and returns the SSH client. See DialDetailed
// for the key and certificate.
func Dial(ctx context.Context, client *miosa.Client, sandboxID string, opts Options) (*ssh.Client, error) {
	res, err := DialDetailed(ctx, client, sandboxID, opts)
	if err != nil {
		return nil, err
	}
	return res.Client, nil
}

// DialDetailed is Dial that also returns the key and certificate.
func DialDetailed(ctx context.Context, client *miosa.Client, sandboxID string, opts Options) (*Result, error) {
	key := opts.Key
	if key == nil {
		var err error
		if key, err = GenerateKey("miosa-" + sandboxID); err != nil {
			return nil, err
		}
	}
	issued, err := client.Sandboxes.IssueSSHCertificate(ctx, sandboxID, miosa.SSHCertificateInput{
		PublicKey:  key.PublicKey,
		TTLSeconds: opts.TTLSeconds,
	})
	if err != nil {
		return nil, err
	}
	conn, err := DialWithCertificate(ctx, client, sandboxID, key, issued, opts)
	if err != nil {
		return nil, err
	}
	return &Result{Client: conn, Key: key, Certificate: issued}, nil
}

// DialWithCertificate connects with a certificate you already hold (from
// IssueSSHCertificate), for example to reconnect before it expires.
func DialWithCertificate(ctx context.Context, client *miosa.Client, sandboxID string, key *KeyPair, issued *miosa.SSHCertificate, opts Options) (*ssh.Client, error) {
	if key == nil || issued == nil {
		return nil, errors.New("sandboxssh: key and certificate are required")
	}
	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(issued.Certificate))
	if err != nil {
		return nil, fmt.Errorf("sandboxssh: parse certificate: %w", err)
	}
	cert, ok := pub.(*ssh.Certificate)
	if !ok {
		return nil, errors.New("sandboxssh: the API returned a plain key where a certificate was expected")
	}
	certSigner, err := ssh.NewCertSigner(cert, key.Signer)
	if err != nil {
		return nil, fmt.Errorf("sandboxssh: certificate does not match the key: %w", err)
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ws, _, err := client.DialWebSocket(dialCtx, "/sandboxes/"+sandboxID+"/ssh-tunnel")
	if err != nil {
		return nil, fmt.Errorf("sandboxssh: open tunnel: %w", err)
	}
	tunnel := NewTunnelConn(ws)

	user := opts.User
	if user == "" {
		user = issued.User
	}
	if user == "" {
		user = "root"
	}
	hostKey := opts.HostKeyCallback
	if hostKey == nil {
		hostKey = ssh.InsecureIgnoreHostKey() //nolint:gosec // documented on Options.HostKeyCallback
	}
	version := opts.ClientVersion
	if version == "" {
		version = "miosa-go"
	}
	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(certSigner)},
		HostKeyCallback: hostKey,
		ClientVersion:   "SSH-2.0-" + version,
		Timeout:         timeout,
	}
	// ssh.NewClientConn has no context; bound the handshake by closing the
	// tunnel when the context ends.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-dialCtx.Done():
			tunnel.Close()
		case <-done:
		}
	}()
	_ = tunnel.SetDeadline(time.Now().Add(timeout))
	sshConn, chans, reqs, err := ssh.NewClientConn(tunnel, sandboxID, cfg)
	if err != nil {
		tunnel.Close()
		return nil, fmt.Errorf("sandboxssh: handshake: %w", err)
	}
	_ = tunnel.SetDeadline(time.Time{})
	return ssh.NewClient(sshConn, chans, reqs), nil
}

// NewTunnelConn adapts a binary WebSocket to a net.Conn: bytes written become
// binary frames and received frames are read as a byte stream. It is how the
// ssh-tunnel and port-tunnel routes are used. Closing it closes the WebSocket.
func NewTunnelConn(ws *websocket.Conn) net.Conn {
	return &tunnelConn{ws: ws}
}

type tunnelConn struct {
	ws *websocket.Conn

	rmu    sync.Mutex
	reader io.Reader

	wmu sync.Mutex
}

func (c *tunnelConn) Read(p []byte) (int, error) {
	c.rmu.Lock()
	defer c.rmu.Unlock()
	for {
		if c.reader == nil {
			mt, r, err := c.ws.NextReader()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					return 0, io.EOF
				}
				return 0, err
			}
			if mt != websocket.BinaryMessage {
				// Text frames are control chatter on this route.
				if _, err := io.Copy(io.Discard, r); err != nil {
					return 0, err
				}
				continue
			}
			c.reader = r
		}
		n, err := c.reader.Read(p)
		if err == io.EOF {
			c.reader = nil
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, err
	}
}

func (c *tunnelConn) Write(p []byte) (int, error) {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if err := c.ws.WriteMessage(websocket.BinaryMessage, bytes.Clone(p)); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *tunnelConn) Close() error {
	c.wmu.Lock()
	_ = c.ws.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
	c.wmu.Unlock()
	return c.ws.Close()
}

func (c *tunnelConn) LocalAddr() net.Addr  { return c.ws.LocalAddr() }
func (c *tunnelConn) RemoteAddr() net.Addr { return c.ws.RemoteAddr() }

func (c *tunnelConn) SetDeadline(t time.Time) error {
	if err := c.ws.SetReadDeadline(t); err != nil {
		return err
	}
	return c.ws.SetWriteDeadline(t)
}
func (c *tunnelConn) SetReadDeadline(t time.Time) error  { return c.ws.SetReadDeadline(t) }
func (c *tunnelConn) SetWriteDeadline(t time.Time) error { return c.ws.SetWriteDeadline(t) }
