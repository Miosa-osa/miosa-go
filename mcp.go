package miosa

import "context"

// McpService provides access to the Model Context Protocol streamable-HTTP
// transport endpoint. Clients (Claude Code, Cursor, Gemini CLI, Copilot) point
// at /api/v1/mcp with a msk_* Bearer token and discover the MIOSA tool-belt.
type McpService struct {
	client *Client
}

// McpDispatchInput is the JSON-RPC request body for POST /mcp.
type McpDispatchInput struct {
	Method string                 `json:"method,omitempty"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// Dispatch sends a JSON-RPC request to the MCP endpoint.
func (s *McpService) Dispatch(ctx context.Context, input McpDispatchInput) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.postJSON(ctx, "/mcp", input, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Listen opens the MCP listen channel (GET /mcp). For true SSE streaming,
// callers should use the underlying HTTP client directly.
func (s *McpService) Listen(ctx context.Context) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.getJSON(ctx, "/mcp", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Close terminates the MCP session (DELETE /mcp).
func (s *McpService) Close(ctx context.Context) error {
	return s.client.deleteJSON(ctx, "/mcp", nil)
}
