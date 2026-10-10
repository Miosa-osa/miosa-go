package miosa

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
)

// The AI Gateway control plane under /ai-gateway: routing policies, budget,
// limits, key environments, settings, request traces and provider health.
// Inference itself (completions, embeddings, models) stays on /intelligence;
// see CompletionsService and friends.
//
// Scopes: intelligence:gateway:read for reads, intelligence:gateway:write for
// every change.

// GatewayPolicy is a routing policy.
type GatewayPolicy struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Selector       map[string]interface{} `json:"selector,omitempty"`
	PrimaryModel   string                 `json:"primary_model"`
	FallbackModels []string               `json:"fallback_models,omitempty"`
	AliasName      string                 `json:"alias_name,omitempty"`
	Strategy       string                 `json:"strategy,omitempty"`
	Weights        map[string]interface{} `json:"weights,omitempty"`
	Priority       int                    `json:"priority"`
	Enabled        bool                   `json:"enabled"`
	CreatedAt      string                 `json:"created_at,omitempty"`
	UpdatedAt      string                 `json:"updated_at,omitempty"`
}

// GatewayPolicyInput is the body of creating a policy.
type GatewayPolicyInput struct {
	Name           string                 `json:"name"`
	Selector       map[string]interface{} `json:"selector,omitempty"`
	PrimaryModel   string                 `json:"primary_model"`
	FallbackModels []string               `json:"fallback_models,omitempty"`
	AliasName      string                 `json:"alias_name,omitempty"`
	Strategy       string                 `json:"strategy,omitempty"`
	Weights        map[string]interface{} `json:"weights,omitempty"`
	Priority       *int                   `json:"priority,omitempty"`
	Enabled        *bool                  `json:"enabled,omitempty"`
}

// GatewayPolicyUpdate changes a policy. Nil members are left alone.
type GatewayPolicyUpdate struct {
	Name           *string                `json:"name,omitempty"`
	Selector       map[string]interface{} `json:"selector,omitempty"`
	PrimaryModel   *string                `json:"primary_model,omitempty"`
	FallbackModels []string               `json:"fallback_models,omitempty"`
	AliasName      *string                `json:"alias_name,omitempty"`
	Strategy       *string                `json:"strategy,omitempty"`
	Weights        map[string]interface{} `json:"weights,omitempty"`
	Priority       *int                   `json:"priority,omitempty"`
	Enabled        *bool                  `json:"enabled,omitempty"`
}

// GatewayBudget is the organization's gateway budget and limits.
type GatewayBudget struct {
	ID                    string `json:"id"`
	MonthlyCreditLimit    *int64 `json:"monthly_credit_limit"`
	PerRequestCreditLimit *int64 `json:"per_request_credit_limit"`
	RequestsPerMinute     *int64 `json:"requests_per_minute"`
	TokensPerDay          *int64 `json:"tokens_per_day"`
	ConsumedCredits       int64  `json:"consumed_credits"`
	HardLimit             bool   `json:"hard_limit"`
	PeriodStart           string `json:"period_start,omitempty"`
	PeriodEnd             string `json:"period_end,omitempty"`
	UpdatedAt             string `json:"updated_at,omitempty"`
}

// GatewayBudgetInput sets the budget. Nil members are omitted.
type GatewayBudgetInput struct {
	MonthlyCreditLimit    *int64 `json:"monthly_credit_limit,omitempty"`
	PerRequestCreditLimit *int64 `json:"per_request_credit_limit,omitempty"`
	RequestsPerMinute     *int64 `json:"requests_per_minute,omitempty"`
	TokensPerDay          *int64 `json:"tokens_per_day,omitempty"`
	HardLimit             *bool  `json:"hard_limit,omitempty"`
}

// GatewayLimit is a limit on one key or environment.
type GatewayLimit struct {
	ScopeType          string `json:"scope_type"`
	ScopeValue         string `json:"scope_value"`
	RequestsPerMinute  *int64 `json:"requests_per_minute"`
	TokensPerDay       *int64 `json:"tokens_per_day"`
	MonthlyCreditLimit *int64 `json:"monthly_credit_limit"`
	HardLimit          bool   `json:"hard_limit"`
	UpdatedAt          string `json:"updated_at,omitempty"`
}

// GatewayLimitInput sets a key or environment limit. Nil members are omitted.
type GatewayLimitInput struct {
	RequestsPerMinute  *int64 `json:"requests_per_minute,omitempty"`
	TokensPerDay       *int64 `json:"tokens_per_day,omitempty"`
	MonthlyCreditLimit *int64 `json:"monthly_credit_limit,omitempty"`
	HardLimit          *bool  `json:"hard_limit,omitempty"`
}

// GatewayLimitScope is the kind of thing a limit applies to.
type GatewayLimitScope string

const (
	GatewayScopeKey         GatewayLimitScope = "key"
	GatewayScopeEnvironment GatewayLimitScope = "environment"
)

// GatewayUsage is current usage in one window.
type GatewayUsage map[string]interface{}

// GatewayLimitsOverview is limits plus current usage for the organization,
// every active key and every environment.
type GatewayLimitsOverview struct {
	Organization map[string]interface{}   `json:"organization"`
	Keys         []map[string]interface{} `json:"keys"`
	Environments []map[string]interface{} `json:"environments"`
	Resets       map[string]interface{}   `json:"resets"`
}

// GatewayEnvironment is the per-environment view: tagged keys, policies that
// target it, limits and usage.
type GatewayEnvironment struct {
	// Environment is production, staging or development.
	Environment string `json:"environment"`
	Keys        []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		KeyPrefix string `json:"key_prefix"`
	} `json:"keys"`
	KeyCount    int                    `json:"key_count"`
	PolicyCount int                    `json:"policy_count"`
	Limits      map[string]interface{} `json:"limits"`
	Usage       map[string]interface{} `json:"usage"`
}

// GatewayKeyEnvironment is the result of tagging a key with an environment.
type GatewayKeyEnvironment struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Environment string `json:"environment"`
}

// GatewaySettings are the organization-level gateway settings.
type GatewaySettings struct {
	DefaultModel  string `json:"default_model"`
	LoggingMode   string `json:"logging_mode"`
	RetentionDays int    `json:"retention_days"`
	MaxRetention  int    `json:"max_retention_days"`
	DefaultClass  string `json:"default_classification"`
	Transcription struct {
		Enabled          bool  `json:"enabled"`
		Explicit         *bool `json:"explicit"`
		DefaultEnabled   bool  `json:"default_enabled"`
		HealthcareTenant bool  `json:"healthcare_tenant"`
	} `json:"transcription"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// GatewaySettingsInput changes settings. Nil members are omitted.
type GatewaySettingsInput struct {
	DefaultModel *string `json:"default_model,omitempty"`
	// LoggingMode is "metadata" or "off".
	LoggingMode *string `json:"logging_mode,omitempty"`
	// RetentionDays is 1 to 30.
	RetentionDays         *int    `json:"retention_days,omitempty"`
	DefaultClassification *string `json:"default_classification,omitempty"`
	// TranscriptionEnabled: set true/false to force; leave nil to keep.
	TranscriptionEnabled *bool `json:"transcription_enabled,omitempty"`
	// ResetTranscription returns transcription to its default (sends null).
	ResetTranscription bool `json:"-"`
	// AcknowledgeNoBAA is required to enable transcription for a healthcare
	// organization: audio may contain PHI and the speech providers have no BAA.
	AcknowledgeNoBAA bool `json:"acknowledge_no_baa,omitempty"`
}

// MarshalJSON implements json.Marshaler.
func (in GatewaySettingsInput) MarshalJSON() ([]byte, error) {
	type plain GatewaySettingsInput
	raw, err := json.Marshal(plain(in))
	if err != nil {
		return nil, err
	}
	if !in.ResetTranscription {
		return raw, nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	m["transcription_enabled"] = nil
	return json.Marshal(m)
}

// GatewayRequestTrace is the metadata of one routed backend attempt. Prompt and
// completion content are never stored.
type GatewayRequestTrace struct {
	ID               string  `json:"id"`
	RequestID        string  `json:"request_id"`
	Attempt          int     `json:"attempt"`
	Operation        string  `json:"operation"`
	RequestedModel   string  `json:"requested_model"`
	Model            string  `json:"model"`
	Provider         string  `json:"provider"`
	APIKeyID         string  `json:"api_key_id"`
	Status           string  `json:"status"`
	HTTPStatus       int     `json:"http_status"`
	ErrorCode        string  `json:"error_code"`
	Stream           bool    `json:"stream"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	CostCredits      float64 `json:"cost_credits"`
	LatencyMS        int64   `json:"latency_ms"`
	CreatedAt        string  `json:"created_at"`
}

// GatewayRequestsOptions filters the trace list.
type GatewayRequestsOptions struct {
	// Status is "success" or "error".
	Status    string
	Provider  string
	Model     string
	APIKeyID  string
	RequestID string
	// From and To are ISO 8601 datetimes.
	From   string
	To     string
	Limit  int
	Cursor string
}

// GatewayRequestList is a page of traces.
type GatewayRequestList struct {
	Data          []GatewayRequestTrace  `json:"data"`
	Summary       map[string]interface{} `json:"summary"`
	NextCursor    string                 `json:"next_cursor,omitempty"`
	RetentionDays int                    `json:"retention_days"`
}

// AIGatewayService is the AI Gateway control plane. Accessed via Client.AIGateway.
type AIGatewayService struct {
	client *Client
}

func gwPath(parts ...string) string {
	p := "/ai-gateway"
	for _, part := range parts {
		p += "/" + url.PathEscape(part)
	}
	return p
}

// ListPolicies returns the routing policies.
func (s *AIGatewayService) ListPolicies(ctx context.Context) ([]GatewayPolicy, error) {
	var out apiResponse[[]GatewayPolicy]
	if err := s.client.getJSON(ctx, gwPath("policies"), &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// GetPolicy returns one routing policy.
func (s *AIGatewayService) GetPolicy(ctx context.Context, id string) (*GatewayPolicy, error) {
	var out apiResponse[GatewayPolicy]
	if err := s.client.getJSON(ctx, gwPath("policies", id), &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// CreatePolicy creates a routing policy.
func (s *AIGatewayService) CreatePolicy(ctx context.Context, in GatewayPolicyInput) (*GatewayPolicy, error) {
	var out apiResponse[GatewayPolicy]
	if err := s.client.postJSON(ctx, gwPath("policies"), in, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// UpdatePolicy changes a routing policy.
func (s *AIGatewayService) UpdatePolicy(ctx context.Context, id string, in GatewayPolicyUpdate) (*GatewayPolicy, error) {
	var out apiResponse[GatewayPolicy]
	if err := s.client.patchJSON(ctx, gwPath("policies", id), in, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// DeletePolicy deletes a routing policy.
func (s *AIGatewayService) DeletePolicy(ctx context.Context, id string) error {
	return s.client.deleteJSON(ctx, gwPath("policies", id), nil)
}

// Budget returns the organization's gateway budget, or nil when none is set.
func (s *AIGatewayService) Budget(ctx context.Context) (*GatewayBudget, error) {
	var out struct {
		Data *GatewayBudget `json:"data"`
	}
	if err := s.client.getJSON(ctx, gwPath("budget"), &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// PutBudget sets the organization's gateway budget.
func (s *AIGatewayService) PutBudget(ctx context.Context, in GatewayBudgetInput) (*GatewayBudget, error) {
	var out apiResponse[GatewayBudget]
	if err := s.client.putJSON(ctx, gwPath("budget"), in, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// Health returns the gateway health readout.
func (s *AIGatewayService) Health(ctx context.Context) (map[string]interface{}, error) {
	var out apiResponse[map[string]interface{}]
	if err := s.client.getJSON(ctx, gwPath("health"), &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// Limits returns limits plus current usage for the organization, every active
// key and every environment.
func (s *AIGatewayService) Limits(ctx context.Context) (*GatewayLimitsOverview, error) {
	var out apiResponse[GatewayLimitsOverview]
	if err := s.client.getJSON(ctx, gwPath("limits"), &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// PutLimit sets the limit of one key or environment. For an environment,
// scopeValue is production, staging or development.
func (s *AIGatewayService) PutLimit(ctx context.Context, scope GatewayLimitScope, scopeValue string, in GatewayLimitInput) (*GatewayLimit, error) {
	var out apiResponse[GatewayLimit]
	if err := s.client.putJSON(ctx, gwPath("limits", string(scope), scopeValue), in, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// DeleteLimit removes the limit of one key or environment.
func (s *AIGatewayService) DeleteLimit(ctx context.Context, scope GatewayLimitScope, scopeValue string) error {
	return s.client.deleteJSON(ctx, gwPath("limits", string(scope), scopeValue), nil)
}

// Environments returns the per-environment overview.
func (s *AIGatewayService) Environments(ctx context.Context) ([]GatewayEnvironment, error) {
	var out apiResponse[[]GatewayEnvironment]
	if err := s.client.getJSON(ctx, gwPath("environments"), &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// SetKeyEnvironment tags a gateway key production, staging or development.
func (s *AIGatewayService) SetKeyEnvironment(ctx context.Context, keyID, environment string) (*GatewayKeyEnvironment, error) {
	if environment == "" {
		return nil, errors.New("environment is required")
	}
	var out apiResponse[GatewayKeyEnvironment]
	body := map[string]string{"environment": environment}
	if err := s.client.putJSON(ctx, gwPath("keys", keyID, "environment"), body, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// Settings returns the organization-level gateway settings.
func (s *AIGatewayService) Settings(ctx context.Context) (*GatewaySettings, error) {
	var out apiResponse[GatewaySettings]
	if err := s.client.getJSON(ctx, gwPath("settings"), &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// UpdateSettings changes the gateway settings. Enabling transcription for a
// healthcare organization needs AcknowledgeNoBAA (else 422
// ACKNOWLEDGEMENT_REQUIRED).
func (s *AIGatewayService) UpdateSettings(ctx context.Context, in GatewaySettingsInput) (*GatewaySettings, error) {
	var out apiResponse[GatewaySettings]
	if err := s.client.putJSON(ctx, gwPath("settings"), in, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// Requests lists request traces, newest first, with a summary of the filtered
// window and a cursor for the next page.
func (s *AIGatewayService) Requests(ctx context.Context, opts GatewayRequestsOptions) (*GatewayRequestList, error) {
	q := map[string]string{
		"status":     opts.Status,
		"provider":   opts.Provider,
		"model":      opts.Model,
		"api_key_id": opts.APIKeyID,
		"request_id": opts.RequestID,
		"from":       opts.From,
		"to":         opts.To,
		"cursor":     opts.Cursor,
	}
	if opts.Limit > 0 {
		q["limit"] = strconv.Itoa(opts.Limit)
	}
	var out GatewayRequestList
	if err := s.client.getJSON(ctx, gwPath("requests")+buildQuery(q), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Request returns one trace.
func (s *AIGatewayService) Request(ctx context.Context, id string) (*GatewayRequestTrace, error) {
	var out apiResponse[GatewayRequestTrace]
	if err := s.client.getJSON(ctx, gwPath("requests", id), &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// ProviderHealth returns per-provider health over a window: "1h", "24h"
// (default) or "7d".
func (s *AIGatewayService) ProviderHealth(ctx context.Context, window string) (map[string]interface{}, error) {
	var out apiResponse[map[string]interface{}]
	if err := s.client.getJSON(ctx, gwPath("provider-health")+buildQuery(map[string]string{"window": window}), &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}
