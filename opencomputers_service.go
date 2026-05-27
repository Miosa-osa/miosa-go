package miosa

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

// ─── OpenComputersService ─────────────────────────────────────────────────────

// OpenComputersService groups all /opencomputers/* sub-services.
// Access via client.OpenComputers.
type OpenComputersService struct {
	client   *Client
	Hosts    *OcHostsService
	Jobs     *OcJobsService
	Files    *OcFilesService
	Terminal *OcTerminalService
	Desktop  *OcDesktopService
	Tunnels  *OcTunnelsService
	Agents   *OcAgentsService
	Clusters *OcClustersService
	Secrets  *OcSecretsService
}

func newOpenComputersService(c *Client) *OpenComputersService {
	return &OpenComputersService{
		client:   c,
		Hosts:    &OcHostsService{client: c},
		Jobs:     &OcJobsService{client: c},
		Files:    &OcFilesService{client: c},
		Terminal: &OcTerminalService{client: c},
		Desktop:  &OcDesktopService{client: c},
		Tunnels:  &OcTunnelsService{client: c},
		Agents:   &OcAgentsService{client: c},
		Clusters: &OcClustersService{client: c},
		Secrets:  &OcSecretsService{client: c},
	}
}

// ─── Hosts ────────────────────────────────────────────────────────────────────

// OcHostsService manages registered OpenComputers hosts.
type OcHostsService struct{ client *Client }

// List returns all hosts for the tenant.
func (s *OcHostsService) List(ctx context.Context) (*HostListResponse, error) {
	var out HostListResponse
	if err := s.client.getJSON(ctx, "/opencomputers/hosts", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Create registers a new host. The host_key in the response is shown once.
func (s *OcHostsService) Create(ctx context.Context, in CreateHostInput) (*HostData, error) {
	var out HostData
	if err := s.client.postJSON(ctx, "/opencomputers/hosts", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get fetches a host by ID.
func (s *OcHostsService) Get(ctx context.Context, id string) (*HostData, error) {
	var out HostData
	if err := s.client.getJSON(ctx, "/opencomputers/hosts/"+id, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update patches a host's name or labels.
func (s *OcHostsService) Update(ctx context.Context, id string, in UpdateHostInput) (*HostData, error) {
	var out HostData
	if err := s.client.patchJSON(ctx, "/opencomputers/hosts/"+id, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Revoke permanently removes a host registration.
func (s *OcHostsService) Revoke(ctx context.Context, id string) error {
	return s.client.deleteJSON(ctx, "/opencomputers/hosts/"+id, nil)
}

// Events streams host state-change events. Each event is sent on the returned
// channel. The channel is closed when the SSE stream ends or ctx is cancelled.
func (s *OcHostsService) Events(ctx context.Context) (<-chan HostEvent, error) {
	return streamSSE[HostEvent](ctx, s.client, "/opencomputers/hosts/events")
}

// ─── Jobs ─────────────────────────────────────────────────────────────────────

// OcJobsService runs and manages exec jobs on remote hosts.
type OcJobsService struct{ client *Client }

// Run dispatches a command to run on the host.
func (s *OcJobsService) Run(ctx context.Context, hostID string, in RunJobInput) (*JobData, error) {
	var out JobData
	if err := s.client.postJSON(ctx, fmt.Sprintf("/opencomputers/hosts/%s/exec", hostID), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// List returns all jobs for a host.
func (s *OcJobsService) List(ctx context.Context, hostID string) (*JobListResponse, error) {
	var out JobListResponse
	if err := s.client.getJSON(ctx, fmt.Sprintf("/opencomputers/hosts/%s/exec", hostID), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get fetches a single job.
func (s *OcJobsService) Get(ctx context.Context, hostID, jobID string) (*JobData, error) {
	var out JobData
	if err := s.client.getJSON(ctx, fmt.Sprintf("/opencomputers/hosts/%s/exec/%s", hostID, jobID), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Stream returns a channel of live output events from a running job.
func (s *OcJobsService) Stream(ctx context.Context, hostID, jobID string) (<-chan JobEvent, error) {
	return streamSSE[JobEvent](ctx, s.client, fmt.Sprintf("/opencomputers/hosts/%s/exec/%s/stream", hostID, jobID))
}

// Cancel cancels a running or queued job.
func (s *OcJobsService) Cancel(ctx context.Context, hostID, jobID string) error {
	return s.client.deleteJSON(ctx, fmt.Sprintf("/opencomputers/hosts/%s/exec/%s", hostID, jobID), nil)
}

// ─── Files ────────────────────────────────────────────────────────────────────

// OcFilesService provides direct file-system access on a remote host.
type OcFilesService struct{ client *Client }

func (s *OcFilesService) fsBase(hostID string) string {
	return fmt.Sprintf("/opencomputers/hosts/%s/fs", hostID)
}

// List lists directory entries at path.
func (s *OcFilesService) List(ctx context.Context, hostID, path string) (*FsListResponse, error) {
	var out FsListResponse
	if err := s.client.getJSON(ctx, s.fsBase(hostID)+buildQuery(map[string]string{"path": path}), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Stat stats a path (lstat — does not follow symlinks).
func (s *OcFilesService) Stat(ctx context.Context, hostID, path string) (*FsStat, error) {
	var out FsStat
	if err := s.client.getJSON(ctx, s.fsBase(hostID)+"/stat"+buildQuery(map[string]string{"path": path}), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Download downloads a file and returns raw bytes.
func (s *OcFilesService) Download(ctx context.Context, hostID, path string) ([]byte, error) {
	data, _, err := s.client.getRaw(ctx, s.fsBase(hostID)+"/download"+buildQuery(map[string]string{"path": path}))
	return data, err
}

// Upload uploads content to remotePath on the host.
func (s *OcFilesService) Upload(ctx context.Context, hostID, remotePath string, content []byte, filename string) (*FsEntry, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("path", remotePath)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("creating form file: %w", err)
	}
	if _, err = fw.Write(content); err != nil {
		return nil, fmt.Errorf("writing file content: %w", err)
	}
	mw.Close()

	var out FsEntry
	if err := s.client.postMultipart(ctx, s.fsBase(hostID)+"/upload", bytes.NewReader(buf.Bytes()), mw.FormDataContentType(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete deletes a file or directory.
func (s *OcFilesService) Delete(ctx context.Context, hostID, path string, recursive bool) error {
	rec := "false"
	if recursive {
		rec = "true"
	}
	return s.client.deleteJSON(ctx, s.fsBase(hostID)+buildQuery(map[string]string{"path": path, "recursive": rec}), nil)
}

// Mkdir creates a directory (including missing parents).
func (s *OcFilesService) Mkdir(ctx context.Context, hostID, path string) error {
	return s.client.postJSON(ctx, s.fsBase(hostID)+"/mkdir", map[string]string{"path": path}, nil)
}

// ─── Terminal ─────────────────────────────────────────────────────────────────

// OcTerminalService issues WS tickets for interactive PTY sessions.
type OcTerminalService struct{ client *Client }

// Ticket issues a short-lived WebSocket ticket for a terminal session.
func (s *OcTerminalService) Ticket(ctx context.Context, hostID string) (*WsTicket, error) {
	var out WsTicket
	if err := s.client.postJSON(ctx, fmt.Sprintf("/opencomputers/hosts/%s/terminal/ticket", hostID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ─── Desktop ─────────────────────────────────────────────────────────────────

// OcDesktopService issues WS tickets for KasmVNC desktop streaming.
type OcDesktopService struct{ client *Client }

// Ticket issues a short-lived WebSocket ticket for a desktop streaming session.
func (s *OcDesktopService) Ticket(ctx context.Context, hostID string) (*WsTicket, error) {
	var out WsTicket
	if err := s.client.postJSON(ctx, fmt.Sprintf("/opencomputers/hosts/%s/desktop/ticket", hostID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ─── Tunnels ──────────────────────────────────────────────────────────────────

// OcTunnelsService manages HTTP tunnels exposing host ports publicly.
type OcTunnelsService struct{ client *Client }

func (s *OcTunnelsService) tunnelBase(hostID string) string {
	return fmt.Sprintf("/opencomputers/hosts/%s/tunnels", hostID)
}

// List returns all tunnels for a host.
func (s *OcTunnelsService) List(ctx context.Context, hostID string) (*TunnelListResponse, error) {
	var out TunnelListResponse
	if err := s.client.getJSON(ctx, s.tunnelBase(hostID), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Create creates a new tunnel.
func (s *OcTunnelsService) Create(ctx context.Context, hostID string, in CreateTunnelInput) (*TunnelData, error) {
	var out TunnelData
	if err := s.client.postJSON(ctx, s.tunnelBase(hostID), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get fetches a single tunnel.
func (s *OcTunnelsService) Get(ctx context.Context, hostID, tunnelID string) (*TunnelData, error) {
	var out TunnelData
	if err := s.client.getJSON(ctx, s.tunnelBase(hostID)+"/"+tunnelID, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update patches a tunnel.
func (s *OcTunnelsService) Update(ctx context.Context, hostID, tunnelID string, in UpdateTunnelInput) (*TunnelData, error) {
	var out TunnelData
	if err := s.client.patchJSON(ctx, s.tunnelBase(hostID)+"/"+tunnelID, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete deletes a tunnel.
func (s *OcTunnelsService) Delete(ctx context.Context, hostID, tunnelID string) error {
	return s.client.deleteJSON(ctx, s.tunnelBase(hostID)+"/"+tunnelID, nil)
}

// ─── Agents ───────────────────────────────────────────────────────────────────

// OcAgentsService dispatches and manages AI agent sessions on remote hosts.
type OcAgentsService struct{ client *Client }

func (s *OcAgentsService) agentBase(hostID string) string {
	return fmt.Sprintf("/opencomputers/hosts/%s/agent", hostID)
}

// Dispatch dispatches a new agent session.
func (s *OcAgentsService) Dispatch(ctx context.Context, hostID string, in DispatchAgentInput) (*OcAgentSession, error) {
	var out OcAgentSession
	if err := s.client.postJSON(ctx, s.agentBase(hostID)+"/dispatch", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// List returns all agent sessions for a host.
func (s *OcAgentsService) List(ctx context.Context, hostID string) (*OcAgentSessionListResponse, error) {
	var out OcAgentSessionListResponse
	if err := s.client.getJSON(ctx, s.agentBase(hostID)+"/sessions", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get fetches a specific agent session.
func (s *OcAgentsService) Get(ctx context.Context, hostID, sessionID string) (*OcAgentSession, error) {
	var out OcAgentSession
	if err := s.client.getJSON(ctx, s.agentBase(hostID)+"/sessions/"+sessionID, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Events streams live events from an agent session.
func (s *OcAgentsService) Events(ctx context.Context, hostID, sessionID string) (<-chan OcAgentEvent, error) {
	return streamSSE[OcAgentEvent](ctx, s.client, s.agentBase(hostID)+"/sessions/"+sessionID+"/events")
}

// Cancel cancels an agent session.
func (s *OcAgentsService) Cancel(ctx context.Context, hostID, sessionID string) error {
	return s.client.deleteJSON(ctx, s.agentBase(hostID)+"/sessions/"+sessionID, nil)
}

// ─── Clusters ─────────────────────────────────────────────────────────────────

// OcClustersService manages multi-host LLM inference clusters.
type OcClustersService struct{ client *Client }

// List returns all clusters for the tenant.
func (s *OcClustersService) List(ctx context.Context) (*ClusterListResponse, error) {
	var out ClusterListResponse
	if err := s.client.getJSON(ctx, "/opencomputers/clusters", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Create provisions a new inference cluster.
func (s *OcClustersService) Create(ctx context.Context, in CreateClusterInput) (*ClusterData, error) {
	var out ClusterData
	if err := s.client.postJSON(ctx, "/opencomputers/clusters", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get fetches a single cluster.
func (s *OcClustersService) Get(ctx context.Context, id string) (*ClusterData, error) {
	var out ClusterData
	if err := s.client.getJSON(ctx, "/opencomputers/clusters/"+id, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Start starts a stopped cluster.
func (s *OcClustersService) Start(ctx context.Context, id string) (*ClusterData, error) {
	var out ClusterData
	if err := s.client.postJSON(ctx, "/opencomputers/clusters/"+id+"/start", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Stop stops a running cluster.
func (s *OcClustersService) Stop(ctx context.Context, id string) (*ClusterData, error) {
	var out ClusterData
	if err := s.client.postJSON(ctx, "/opencomputers/clusters/"+id+"/stop", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete permanently deletes a cluster.
func (s *OcClustersService) Delete(ctx context.Context, id string) error {
	return s.client.deleteJSON(ctx, "/opencomputers/clusters/"+id, nil)
}

// ─── Secrets ──────────────────────────────────────────────────────────────────

// OcSecretsService manages encrypted per-host / per-tenant secrets.
type OcSecretsService struct{ client *Client }

// ListForTenant lists all tenant-scoped secrets.
func (s *OcSecretsService) ListForTenant(ctx context.Context) ([]SecretData, error) {
	var out struct {
		Data []SecretData `json:"data"`
	}
	if err := s.client.getJSON(ctx, "/opencomputers/secrets", &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// CreateForTenant creates a tenant-scoped secret.
func (s *OcSecretsService) CreateForTenant(ctx context.Context, in CreateSecretInput) (*SecretData, error) {
	var out SecretData
	if err := s.client.postJSON(ctx, "/opencomputers/secrets", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListForHost lists secrets scoped to a host.
func (s *OcSecretsService) ListForHost(ctx context.Context, hostID string) ([]SecretData, error) {
	var out struct {
		Data []SecretData `json:"data"`
	}
	if err := s.client.getJSON(ctx, fmt.Sprintf("/opencomputers/hosts/%s/secrets", hostID), &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// CreateForHost creates a host-scoped secret.
func (s *OcSecretsService) CreateForHost(ctx context.Context, hostID string, in CreateSecretInput) (*SecretData, error) {
	var out SecretData
	if err := s.client.postJSON(ctx, fmt.Sprintf("/opencomputers/hosts/%s/secrets", hostID), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateForHost rotates the value or updates the description of a host-scoped secret.
func (s *OcSecretsService) UpdateForHost(ctx context.Context, hostID, secretID string, in UpdateSecretInput) (*SecretData, error) {
	var out SecretData
	if err := s.client.patchJSON(ctx, fmt.Sprintf("/opencomputers/hosts/%s/secrets/%s", hostID, secretID), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteForHost deletes a host-scoped secret.
func (s *OcSecretsService) DeleteForHost(ctx context.Context, hostID, secretID string) error {
	return s.client.deleteJSON(ctx, fmt.Sprintf("/opencomputers/hosts/%s/secrets/%s", hostID, secretID), nil)
}

// Reveal decrypts and returns the plaintext value. Each call is audit-logged.
func (s *OcSecretsService) Reveal(ctx context.Context, hostID, secretID string) (string, error) {
	var out struct {
		Value string `json:"value"`
	}
	if err := s.client.postJSON(ctx, fmt.Sprintf("/opencomputers/hosts/%s/secrets/%s/reveal", hostID, secretID), nil, &out); err != nil {
		return "", err
	}
	return out.Value, nil
}

// ─── SSE streaming helper ─────────────────────────────────────────────────────

// streamSSE opens a GET SSE stream and fans events of type T onto the returned
// channel. The channel is closed when the stream ends or ctx is cancelled.
func streamSSE[T any](ctx context.Context, c *Client, path string) (<-chan T, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("building SSE request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("User-Agent", "miosa-go/"+sdkVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("opening SSE stream: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, errorFromResponse(resp)
	}

	ch := make(chan T, 16)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		consumeSSE(ctx, resp.Body, ch)
	}()
	return ch, nil
}

// consumeSSE reads SSE lines from r and sends parsed events onto ch.
func consumeSSE[T any](ctx context.Context, r io.Reader, ch chan<- T) {
	scanner := bufio.NewScanner(r)
	var dataLines []string

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()

		if line == "" {
			// blank line = dispatch event
			if len(dataLines) > 0 {
				raw := strings.Join(dataLines, "\n")
				if raw == "[DONE]" {
					return
				}
				var ev T
				if err := json.Unmarshal([]byte(raw), &ev); err == nil {
					select {
					case ch <- ev:
					case <-ctx.Done():
						return
					}
				}
				dataLines = dataLines[:0]
			}
			continue
		}

		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(line[5:]))
		}
		// event:, id:, retry: fields — ignore for now
	}
}
