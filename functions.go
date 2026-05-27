package miosa

import "context"

// FunctionsService provides CRUD and invocation for edge functions.
type FunctionsService struct {
	client *Client
}

// ─── Types ────────────────────────────────────────────────────────────────────

// FunctionData is the API representation of an edge function.
type FunctionData struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	TenantID  string `json:"tenant_id"`
	Runtime   string `json:"runtime,omitempty"`
	Status    string `json:"status"`
	InvokeURL string `json:"invoke_url,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// FunctionListResponse wraps GET /functions.
type FunctionListResponse struct {
	Data []FunctionData `json:"data"`
}

// CreateFunctionInput is the request body for POST /functions.
type CreateFunctionInput struct {
	Name           string                 `json:"name"`
	Runtime        string                 `json:"runtime,omitempty"`
	Handler        string                 `json:"handler,omitempty"`
	Code           string                 `json:"code,omitempty"`
	Env            map[string]string      `json:"env,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	IdempotencyKey string                 `json:"-"`
}

// UpdateFunctionInput is the request body for PATCH /functions/:id.
type UpdateFunctionInput struct {
	Name     string                 `json:"name,omitempty"`
	Handler  string                 `json:"handler,omitempty"`
	Code     string                 `json:"code,omitempty"`
	Env      map[string]string      `json:"env,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// InvokeFunctionInput is the request body for POST /functions/:id/invoke.
type InvokeFunctionInput struct {
	Payload map[string]interface{} `json:"payload,omitempty"`
	Headers map[string]string      `json:"-"` // forwarded as request headers, not body
}

// InvokeFunctionResult is the response from POST /functions/:id/invoke.
type InvokeFunctionResult struct {
	StatusCode int                    `json:"status_code"`
	Body       map[string]interface{} `json:"body,omitempty"`
	Headers    map[string]string      `json:"headers,omitempty"`
	DurationMS int64                  `json:"duration_ms,omitempty"`
}

// ListFunctionsInput holds optional query parameters for GET /functions.
type ListFunctionsInput struct {
	Status string
	Limit  int
	Cursor string
}

// ─── Methods ──────────────────────────────────────────────────────────────────

// List returns edge functions for the authenticated tenant.
func (s *FunctionsService) List(ctx context.Context, input ListFunctionsInput) (*FunctionListResponse, error) {
	params := map[string]string{}
	if input.Status != "" {
		params["status"] = input.Status
	}
	if input.Cursor != "" {
		params["cursor"] = input.Cursor
	}
	var out FunctionListResponse
	if err := s.client.getJSON(ctx, "/functions"+buildQuery(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get fetches a single edge function by ID.
func (s *FunctionsService) Get(ctx context.Context, id string) (*FunctionData, error) {
	var env apiResponse[FunctionData]
	if err := s.client.getJSON(ctx, "/functions/"+id, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Create provisions a new edge function.
func (s *FunctionsService) Create(ctx context.Context, input CreateFunctionInput) (*FunctionData, error) {
	var env apiResponse[FunctionData]
	if err := s.client.postJSONIdempotent(ctx, "/functions", input, &env, idemKey(input.IdempotencyKey)); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Update patches an existing edge function.
func (s *FunctionsService) Update(ctx context.Context, id string, input UpdateFunctionInput) (*FunctionData, error) {
	var env apiResponse[FunctionData]
	if err := s.client.patchJSON(ctx, "/functions/"+id, input, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Delete removes an edge function.
func (s *FunctionsService) Delete(ctx context.Context, id string) error {
	return s.client.deleteJSON(ctx, "/functions/"+id, nil)
}

// Invoke calls an edge function synchronously and returns the result.
func (s *FunctionsService) Invoke(ctx context.Context, id string, input InvokeFunctionInput) (*InvokeFunctionResult, error) {
	var out InvokeFunctionResult
	payload := input.Payload
	if payload == nil {
		payload = map[string]interface{}{}
	}
	if err := s.client.postJSON(ctx, "/functions/"+id+"/invoke", payload, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
