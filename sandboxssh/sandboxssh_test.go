package sandboxssh_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"

	miosa "github.com/Miosa-osa/miosa-go/v2"
	"github.com/Miosa-osa/miosa-go/v2/sandboxssh"
)

// fakePlatform stands in for the API: it signs certificates with a CA and
// relays the tunnel into an in-process sshd that trusts only that CA.
type fakePlatform struct {
	t         *testing.T
	ca        ssh.Signer
	hostKey   ssh.Signer
	server    *httptest.Server
	mu        sync.Mutex
	certReqs  []map[string]interface{}
	tunnelHdr http.Header
	certTweak func(*ssh.Certificate)
	loginUser string // what the sshd saw
	clientVer string
}

func newFakePlatform(t *testing.T) *fakePlatform {
	t.Helper()
	p := &fakePlatform{t: t, ca: mustSigner(t), hostKey: mustSigner(t)}

	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx_1/ssh-certificates", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		p.mu.Lock()
		p.certReqs = append(p.certReqs, body)
		p.mu.Unlock()
		pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(body["public_key"].(string)))
		if err != nil {
			http.Error(w, `{"error":{"code":"INVALID_PUBLIC_KEY","message":"bad key"}}`, 400)
			return
		}
		ttl := 15 * time.Minute
		if v, ok := body["ttl_seconds"].(float64); ok {
			ttl = time.Duration(v) * time.Second
		}
		cert := &ssh.Certificate{
			Key:             pub,
			Serial:          7,
			CertType:        ssh.UserCert,
			KeyId:           "test",
			ValidPrincipals: []string{"sandbox-sbx_1"},
			ValidAfter:      uint64(time.Now().Add(-time.Minute).Unix()),
			ValidBefore:     uint64(time.Now().Add(ttl).Unix()),
		}
		if p.certTweak != nil {
			p.certTweak(cert)
		}
		if err := cert.SignCert(rand.Reader, p.ca); err != nil {
			t.Errorf("sign: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": map[string]interface{}{
			"certificate": string(ssh.MarshalAuthorizedKey(cert)),
			"principal":   "sandbox-sbx_1", "user": "root", "ttl_seconds": int(ttl.Seconds()),
			"sandbox_id": "sbx_1", "transport": "websocket",
		}})
	})
	mux.HandleFunc("/sandboxes/sbx_1/ssh-tunnel", func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		p.tunnelHdr = r.Header.Clone()
		p.mu.Unlock()
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		go p.serveSSH(sandboxssh.NewTunnelConn(ws))
	})
	p.server = httptest.NewServer(mux)
	t.Cleanup(p.server.Close)
	return p
}

func (p *fakePlatform) client(opts ...miosa.ClientOption) *miosa.Client {
	all := append([]miosa.ClientOption{miosa.WithBaseURL(p.server.URL), miosa.WithMaxRetries(0)}, opts...)
	return miosa.NewClient("msk_u_ssh", all...)
}

func mustSigner(t *testing.T) ssh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	s, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// serveSSH runs a minimal sshd on conn: certificate auth against the CA for the
// principal sandbox-sbx_1, and an exec handler.
func (p *fakePlatform) serveSSH(conn net.Conn) {
	checker := &ssh.CertChecker{
		IsUserAuthority: func(auth ssh.PublicKey) bool {
			return bytes.Equal(auth.Marshal(), p.ca.PublicKey().Marshal())
		},
	}
	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(meta ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			cert, ok := key.(*ssh.Certificate)
			if !ok {
				return nil, fmt.Errorf("a certificate is required")
			}
			// Principal is the sandbox, never the login user.
			if err := checker.CheckCert("sandbox-sbx_1", cert); err != nil {
				return nil, err
			}
			p.mu.Lock()
			p.loginUser = meta.User()
			p.clientVer = string(meta.ClientVersion())
			p.mu.Unlock()
			return &ssh.Permissions{}, nil
		},
	}
	cfg.AddHostKey(p.hostKey)
	sconn, chans, reqs, err := ssh.NewServerConn(conn, cfg)
	if err != nil {
		conn.Close()
		return
	}
	go ssh.DiscardRequests(reqs)
	for ch := range chans {
		if ch.ChannelType() != "session" {
			_ = ch.Reject(ssh.UnknownChannelType, "no")
			continue
		}
		channel, creqs, err := ch.Accept()
		if err != nil {
			continue
		}
		go func() {
			defer channel.Close()
			for req := range creqs {
				if req.Type != "exec" {
					_ = req.Reply(false, nil)
					continue
				}
				var payload struct{ Command string }
				_ = ssh.Unmarshal(req.Payload, &payload)
				_ = req.Reply(true, nil)
				_, _ = channel.Write([]byte("ran: " + payload.Command + " as " + sconn.User()))
				_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
				return
			}
		}()
	}
}

// proxyTo forwards a request to the fake platform, so a test can build a server
// that has the certificate route but a different tunnel.
func proxyTo(p *fakePlatform) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, err := http.Post(p.server.URL+r.URL.Path, "application/json", r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(resp.Body)
		_, _ = w.Write(buf.Bytes())
	})
}

func TestGenerateKeyRoundTrips(t *testing.T) {
	k, err := sandboxssh.GenerateKey("ci@example")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(k.PublicKey, "ssh-ed25519 ") || !strings.HasSuffix(k.PublicKey, " ci@example") {
		t.Errorf("public key = %q", k.PublicKey)
	}
	if !strings.Contains(string(k.PrivateKeyPEM), "BEGIN OPENSSH PRIVATE KEY") {
		t.Errorf("private key is not OpenSSH PEM: %.40s", k.PrivateKeyPEM)
	}
	back, err := sandboxssh.ParsePrivateKey(k.PrivateKeyPEM)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(back.Signer.PublicKey().Marshal(), k.Signer.PublicKey().Marshal()) {
		t.Error("a parsed key must be the same key")
	}
	if !strings.HasPrefix(back.PublicKey, "ssh-ed25519 ") {
		t.Errorf("parsed public key = %q", back.PublicKey)
	}
	if _, err := sandboxssh.ParsePrivateKey([]byte("not a key")); err == nil {
		t.Error("garbage parsed")
	}
	other, _ := sandboxssh.GenerateKey("")
	if other.PublicKey == k.PublicKey {
		t.Error("keys must be unique")
	}
	if strings.Contains(other.PublicKey, " ") && strings.Count(other.PublicKey, " ") != 1 {
		t.Errorf("no comment means no trailing text: %q", other.PublicKey)
	}
}

func TestDialConnectsThroughTheTunnelWithACertificate(t *testing.T) {
	p := newFakePlatform(t)

	conn, err := sandboxssh.Dial(context.Background(), p.client(), "sbx_1", sandboxssh.Options{TTLSeconds: 600})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	out, err := session.Output("uname -a")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "ran: uname -a as root" {
		t.Errorf("output = %q", out)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.certReqs) != 1 {
		t.Fatalf("certificate requests = %d", len(p.certReqs))
	}
	req := p.certReqs[0]
	if !strings.HasPrefix(req["public_key"].(string), "ssh-ed25519 ") || req["ttl_seconds"] != float64(600) {
		t.Errorf("certificate request = %v", req)
	}
	if got := p.tunnelHdr.Get("Authorization"); got != "Bearer msk_u_ssh" {
		t.Errorf("tunnel Authorization = %q", got)
	}
	if p.loginUser != "root" {
		t.Errorf("login user = %q", p.loginUser)
	}
	if p.clientVer != "SSH-2.0-miosa-go" {
		t.Errorf("client version = %q", p.clientVer)
	}
}

func TestDialDetailedReturnsKeyAndCertificateForReuse(t *testing.T) {
	p := newFakePlatform(t)
	client := p.client()

	res, err := sandboxssh.DialDetailed(context.Background(), client, "sbx_1", sandboxssh.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res.Client.Close()
	if res.Certificate.Principal != "sandbox-sbx_1" || res.Key == nil || res.Key.PublicKey == "" {
		t.Fatalf("result = %+v", res)
	}

	again, err := sandboxssh.DialWithCertificate(context.Background(), client, "sbx_1", res.Key, res.Certificate, sandboxssh.Options{})
	if err != nil {
		t.Fatalf("reconnecting with the same certificate: %v", err)
	}
	again.Close()
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.certReqs) != 1 {
		t.Errorf("a reconnect must not ask for another certificate, got %d requests", len(p.certReqs))
	}
}

func TestDialWithOwnKeyUserAndVersion(t *testing.T) {
	p := newFakePlatform(t)
	key, err := sandboxssh.GenerateKey("platform")
	if err != nil {
		t.Fatal(err)
	}

	conn, err := sandboxssh.Dial(context.Background(), p.client(), "sbx_1", sandboxssh.Options{
		Key: key, User: "dev", ClientVersion: "acme-ide_1.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.certReqs[0]["public_key"] != key.PublicKey {
		t.Errorf("the supplied key must be the one certified: %v", p.certReqs[0]["public_key"])
	}
	if p.loginUser != "dev" || p.clientVer != "SSH-2.0-acme-ide_1.0" {
		t.Errorf("user=%q version=%q", p.loginUser, p.clientVer)
	}
}

func TestDialHostKeyCallbackIsHonored(t *testing.T) {
	p := newFakePlatform(t)
	wrong := mustSigner(t).PublicKey()

	_, err := sandboxssh.Dial(context.Background(), p.client(), "sbx_1", sandboxssh.Options{
		HostKeyCallback: ssh.FixedHostKey(wrong),
	})
	if err == nil {
		t.Fatal("a pinned host key that does not match must fail the handshake")
	}

	conn, err := sandboxssh.Dial(context.Background(), p.client(), "sbx_1", sandboxssh.Options{
		HostKeyCallback: ssh.FixedHostKey(p.hostKey.PublicKey()),
	})
	if err != nil {
		t.Fatalf("matching pinned host key: %v", err)
	}
	conn.Close()
}

func TestDialRejectsACertificateTheSandboxDoesNotTrust(t *testing.T) {
	p := newFakePlatform(t)
	p.certTweak = func(c *ssh.Certificate) { c.ValidPrincipals = []string{"sandbox-other"} }

	if _, err := sandboxssh.Dial(context.Background(), p.client(), "sbx_1", sandboxssh.Options{}); err == nil {
		t.Fatal("a certificate for another sandbox's principal must be refused")
	}
}

func TestDialSurfacesAPIErrors(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx_1/ssh-certificates", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(409)
		_, _ = w.Write([]byte(`{"error":{"code":"SANDBOX_NOT_RUNNING","message":"sandbox must be running"}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := miosa.NewClient("k", miosa.WithBaseURL(srv.URL), miosa.WithMaxRetries(0))

	_, err := sandboxssh.Dial(context.Background(), client, "sbx_1", sandboxssh.Options{})

	if !miosa.IsCode(err, "SANDBOX_NOT_RUNNING") {
		t.Fatalf("error = %v", err)
	}
}

func TestDialFailsWhenTheTunnelIsRefused(t *testing.T) {
	p := newFakePlatform(t)
	// Point at a server with the certificate route but no tunnel.
	mux := http.NewServeMux()
	mux.Handle("/sandboxes/sbx_1/ssh-certificates", proxyTo(p))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := miosa.NewClient("k", miosa.WithBaseURL(srv.URL), miosa.WithMaxRetries(0))

	_, err := sandboxssh.Dial(context.Background(), client, "sbx_1", sandboxssh.Options{})

	if err == nil || !strings.Contains(err.Error(), "open tunnel") {
		t.Fatalf("error = %v", err)
	}
}

func TestDialHonorsContextCancellation(t *testing.T) {
	// A tunnel that accepts the WebSocket and never speaks SSH.
	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	p := newFakePlatform(t)
	mux.Handle("/sandboxes/sbx_1/ssh-certificates", proxyTo(p))
	mux.HandleFunc("/sandboxes/sbx_1/ssh-tunnel", func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		time.Sleep(3 * time.Second)
		ws.Close()
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := miosa.NewClient("k", miosa.WithBaseURL(srv.URL), miosa.WithMaxRetries(0))

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := sandboxssh.Dial(ctx, client, "sbx_1", sandboxssh.Options{})

	if err == nil {
		t.Fatal("expected the dial to fail")
	}
	if time.Since(start) > 2*time.Second {
		t.Errorf("the handshake ignored the context for %v", time.Since(start))
	}
}

func TestTenantAndUserAgentReachTheTunnel(t *testing.T) {
	p := newFakePlatform(t)
	client := p.client(miosa.WithTenant("org-9"), miosa.WithUserAgentOverride("acme-ide/1.0"))

	conn, err := sandboxssh.Dial(context.Background(), client, "sbx_1", sandboxssh.Options{})
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.tunnelHdr.Get(miosa.HeaderTenant) != "org-9" || p.tunnelHdr.Get("User-Agent") != "acme-ide/1.0" {
		t.Errorf("tunnel headers = %v", p.tunnelHdr)
	}
}
