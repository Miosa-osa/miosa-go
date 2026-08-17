package miosa

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type ForgeRepository struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Slug          string   `json:"slug"`
	DefaultBranch string   `json:"default_branch"`
	Visibility    string   `json:"visibility"`
	State         string   `json:"state"`
	CloneReady    bool     `json:"clone_ready"`
	CloneURL      *string  `json:"clone_url"`
	ProjectIDs    []string `json:"project_ids"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
}
type CreateForgeRepositoryInput struct {
	Name           string   `json:"name"`
	Slug           string   `json:"slug,omitempty"`
	DefaultBranch  string   `json:"default_branch,omitempty"`
	Visibility     string   `json:"visibility,omitempty"`
	ProjectIDs     []string `json:"project_ids,omitempty"`
	IdempotencyKey string   `json:"-"`
}
type UpdateForgeRepositoryInput struct {
	Name       string   `json:"name,omitempty"`
	Slug       string   `json:"slug,omitempty"`
	Visibility string   `json:"visibility,omitempty"`
	ProjectIDs []string `json:"project_ids,omitempty"`
}
type ForgeDeleteReceipt struct {
	OperationID string `json:"operation_id"`
	Replayed    bool   `json:"replayed"`
}
type ForgeNamedRef struct {
	Name string `json:"name"`
	OID  string `json:"oid"`
}
type ForgeBranch struct {
	ForgeNamedRef
	IsDefault bool `json:"is_default"`
}
type ForgeRepositoryRefs struct {
	DefaultBranch string          `json:"default_branch"`
	HeadOID       *string         `json:"head_oid"`
	Branches      []ForgeBranch   `json:"branches"`
	Tags          []ForgeNamedRef `json:"tags"`
}
type ForgeTreeEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
	OID  string `json:"oid"`
	Size *int64 `json:"size"`
}
type ForgeRepositoryTree struct {
	Ref       string           `json:"ref"`
	CommitOID string           `json:"commit_oid"`
	Path      string           `json:"path"`
	Entries   []ForgeTreeEntry `json:"entries"`
	Truncated bool             `json:"truncated"`
}
type ForgeRepositoryBlob struct {
	Ref       string `json:"ref"`
	CommitOID string `json:"commit_oid"`
	Path      string `json:"path"`
	OID       string `json:"oid"`
	Size      int64  `json:"size"`
	Encoding  string `json:"encoding"`
	Content   string `json:"content"`
}
type ForgeCommit struct {
	OID           string   `json:"oid"`
	ShortOID      string   `json:"short_oid"`
	Subject       string   `json:"subject"`
	AuthorName    string   `json:"author_name"`
	AuthorEmail   string   `json:"author_email"`
	AuthoredAt    string   `json:"authored_at"`
	CommitterName string   `json:"committer_name"`
	CommittedAt   string   `json:"committed_at"`
	Parents       []string `json:"parents"`
}
type ForgeCommitPage struct {
	HasMore    bool    `json:"has_more"`
	NextCursor *string `json:"next_cursor"`
}
type ForgeCommitHistory struct {
	Ref     string          `json:"ref"`
	Path    string          `json:"path"`
	Commits []ForgeCommit   `json:"commits"`
	Page    ForgeCommitPage `json:"page"`
}
type ForgeContentOptions struct {
	Ref  string
	Path string
}
type ForgeCommitOptions struct {
	Ref    string
	Path   string
	Limit  int
	Cursor string
}
type ForgeService struct{ client *Client }

func (s *ForgeService) Create(ctx context.Context, input CreateForgeRepositoryInput) (*ForgeRepository, error) {
	if input.IdempotencyKey == "" {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		input.IdempotencyKey = "forge-sdk-" + hex.EncodeToString(b)
	}
	return s.mutate(ctx, http.MethodPost, "/forge/repositories", input, input.IdempotencyKey)
}
func (s *ForgeService) List(ctx context.Context) ([]ForgeRepository, error) {
	var e struct {
		Data []ForgeRepository `json:"data"`
	}
	if err := s.client.getJSON(ctx, "/forge/repositories", &e); err != nil {
		return nil, err
	}
	for i := range e.Data {
		if err := validateForgeRepository(&e.Data[i]); err != nil {
			return nil, err
		}
	}
	return e.Data, nil
}
func (s *ForgeService) Get(ctx context.Context, id string) (*ForgeRepository, error) {
	var e struct {
		Data ForgeRepository `json:"data"`
	}
	if err := s.client.getJSON(ctx, "/forge/repositories/"+url.PathEscape(id), &e); err != nil {
		return nil, err
	}
	return &e.Data, validateForgeRepository(&e.Data)
}
func (s *ForgeService) Refs(ctx context.Context, id string) (*ForgeRepositoryRefs, error) {
	var e struct {
		Data ForgeRepositoryRefs `json:"data"`
	}
	if err := s.client.getJSON(ctx, forgeRepositoryPath(id)+"/refs", &e); err != nil {
		return nil, err
	}
	if e.Data.DefaultBranch == "" || e.Data.Branches == nil || e.Data.Tags == nil {
		return nil, fmt.Errorf("miosa: Forge refs response did not match contract")
	}
	return &e.Data, nil
}
func (s *ForgeService) Tree(ctx context.Context, id string, opts ForgeContentOptions) (*ForgeRepositoryTree, error) {
	var e struct {
		Data ForgeRepositoryTree `json:"data"`
	}
	if err := s.client.getJSON(ctx, forgeContentPath(id, "tree", opts.Ref, opts.Path, 0, ""), &e); err != nil {
		return nil, err
	}
	if e.Data.Ref == "" || e.Data.CommitOID == "" || e.Data.Entries == nil {
		return nil, fmt.Errorf("miosa: Forge tree response did not match contract")
	}
	for _, item := range e.Data.Entries {
		if item.Type != "blob" && item.Type != "tree" {
			return nil, fmt.Errorf("miosa: Forge tree entry has invalid type")
		}
	}
	return &e.Data, nil
}
func (s *ForgeService) Blob(ctx context.Context, id string, opts ForgeContentOptions) (*ForgeRepositoryBlob, error) {
	return s.blob(ctx, id, "blob", opts)
}
func (s *ForgeService) Readme(ctx context.Context, id string, opts ForgeContentOptions) (*ForgeRepositoryBlob, error) {
	return s.blob(ctx, id, "readme", opts)
}
func (s *ForgeService) blob(ctx context.Context, id, endpoint string, opts ForgeContentOptions) (*ForgeRepositoryBlob, error) {
	var e struct {
		Data ForgeRepositoryBlob `json:"data"`
	}
	if err := s.client.getJSON(ctx, forgeContentPath(id, endpoint, opts.Ref, opts.Path, 0, ""), &e); err != nil {
		return nil, err
	}
	if e.Data.Ref == "" || e.Data.CommitOID == "" || e.Data.Path == "" || e.Data.OID == "" || e.Data.Size < 0 || (e.Data.Encoding != "utf-8" && e.Data.Encoding != "base64") {
		return nil, fmt.Errorf("miosa: Forge blob response did not match contract")
	}
	return &e.Data, nil
}
func (s *ForgeService) Commits(ctx context.Context, id string, opts ForgeCommitOptions) (*ForgeCommitHistory, error) {
	var e struct {
		Data ForgeCommitHistory `json:"data"`
	}
	if err := s.client.getJSON(ctx, forgeContentPath(id, "commits", opts.Ref, opts.Path, opts.Limit, opts.Cursor), &e); err != nil {
		return nil, err
	}
	if e.Data.Ref == "" || e.Data.Commits == nil {
		return nil, fmt.Errorf("miosa: Forge commit history response did not match contract")
	}
	for _, commit := range e.Data.Commits {
		if commit.OID == "" || commit.ShortOID == "" || commit.Parents == nil {
			return nil, fmt.Errorf("miosa: Forge commit response did not match contract")
		}
	}
	return &e.Data, nil
}
func (s *ForgeService) Update(ctx context.Context, id string, input UpdateForgeRepositoryInput) (*ForgeRepository, error) {
	return s.mutate(ctx, http.MethodPatch, "/forge/repositories/"+url.PathEscape(id), input, "")
}
func (s *ForgeService) Delete(ctx context.Context, id string) (*ForgeDeleteReceipt, error) {
	resp, err := s.client.do(ctx, http.MethodDelete, "/forge/repositories/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	op := resp.Header.Get("X-Forge-Operation-Id")
	if op == "" {
		return nil, fmt.Errorf("miosa: Forge delete omitted operation receipt")
	}
	return &ForgeDeleteReceipt{OperationID: op, Replayed: resp.Header.Get("Idempotency-Replayed") == "true"}, nil
}
func (s *ForgeService) mutate(ctx context.Context, method, path string, input interface{}, key string) (*ForgeRepository, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	headers := map[string]string{}
	if key != "" {
		headers["Idempotency-Key"] = key
	}
	resp, err := s.client.doWithHeaders(ctx, method, path, bytes.NewReader(body), headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var e struct {
		Data ForgeRepository `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&e); err != nil {
		return nil, err
	}
	return &e.Data, validateForgeRepository(&e.Data)
}
func validateForgeRepository(r *ForgeRepository) error {
	if r.ID == "" || r.Name == "" || r.Slug == "" || r.DefaultBranch == "" || r.CreatedAt == "" || r.UpdatedAt == "" || r.ProjectIDs == nil {
		return fmt.Errorf("miosa: Forge repository response did not match contract")
	}
	if r.Visibility != "public" && r.Visibility != "private" && r.Visibility != "internal" {
		return fmt.Errorf("miosa: invalid Forge visibility")
	}
	active := r.State == "active"
	if r.State != "provisioning" && !active && r.State != "error" && r.State != "deletion_pending" && r.State != "deleted" {
		return fmt.Errorf("miosa: invalid Forge state")
	}
	if r.CloneReady != active || (active && (r.CloneURL == nil || *r.CloneURL == "")) || (!active && r.CloneURL != nil) {
		return fmt.Errorf("miosa: inconsistent Forge clone readiness")
	}
	return nil
}

func forgeRepositoryPath(id string) string { return "/forge/repositories/" + url.PathEscape(id) }
func forgeContentPath(id, endpoint, ref, path string, limit int, cursor string) string {
	values := url.Values{}
	if ref != "" {
		values.Set("ref", ref)
	}
	if path != "" {
		values.Set("path", path)
	}
	if limit > 0 {
		values.Set("limit", fmt.Sprintf("%d", limit))
	}
	if cursor != "" {
		values.Set("cursor", cursor)
	}
	result := forgeRepositoryPath(id) + "/" + endpoint
	if encoded := values.Encode(); encoded != "" {
		result += "?" + encoded
	}
	return result
}
