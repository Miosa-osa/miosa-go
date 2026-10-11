// Package computeruse adapts a MIOSA computer to Anthropic's computer-use
// toolset, so a Claude tool loop can drive the desktop.
//
//	ts, err := computeruse.New(computer, computeruse.Config{
//	    Confirm: func(ctx context.Context, c computeruse.Call) (bool, error) { return true, nil },
//	})
//	tools := ts.Tools() // [{"type":"computer_toolset_20260801"}]
//	// ... send tools to the Messages API, then for each tool_use block:
//	results := ts.ExecuteBatch(ctx, toolUses)
//
// The adapter implements the 17 members of computer_toolset_20260801, echoes
// toolset_name on every result, stops a batch at its first failure and answers
// the rest with the exact skipped-action text. The older computer_20241022,
// computer_20250124 and computer_20251124 tools are served by Legacy.
package computeruse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	miosa "github.com/Miosa-osa/miosa-go/v2"
)

// ToolsetType is the current GA toolset.
const ToolsetType = "computer_toolset_20260801"

// ToolsetName is echoed on every result.
const ToolsetName = "computer"

// SkippedText is the exact result for actions after a failed one in a batch.
const SkippedText = "Not executed: an earlier computer action in this turn failed."

// maxErrorText is the longest error text returned to the model.
const maxErrorText = 4096

// Members lists the toolset's members in the documented order.
var Members = miosa.Actions

// Driver is what the adapter needs from a computer. *miosa.Computer satisfies it.
type Driver interface {
	Act(ctx context.Context, action string, p miosa.ActionParams) (*miosa.ActionResult, error)
}

// Call is one requested action, passed to Config.Confirm.
type Call struct {
	Member string
	Input  miosa.ActionParams
}

// Config configures a Toolset.
type Config struct {
	// Confirm approves actions that type, press or hold keys. It is required
	// unless those members are in Disabled, mirroring Anthropic's rule.
	Confirm func(ctx context.Context, c Call) (bool, error)
	// Disabled removes members from the toolset; a call to one is an error.
	Disabled []string
}

// ErrConfirmRequired is returned by New when type, key or hold_key are enabled
// without a Confirm callback.
var ErrConfirmRequired = errors.New("computeruse: Config.Confirm is required while type, key or hold_key are enabled (or disable them)")

// needsConfirm are the members that put input into the machine.
var needsConfirm = map[string]bool{"type": true, "key": true, "hold_key": true}

// ToolUse is a tool_use block from the model.
type ToolUse struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ImageSource is a base64 image.
type ImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

// Content is one block of a tool result.
type Content struct {
	Type   string       `json:"type"`
	Text   string       `json:"text,omitempty"`
	Source *ImageSource `json:"source,omitempty"`
}

// ToolResult is a tool_result block, ready to place in the next user message.
type ToolResult struct {
	Type        string    `json:"type"`
	ToolUseID   string    `json:"tool_use_id"`
	ToolsetName string    `json:"toolset_name,omitempty"`
	IsError     bool      `json:"is_error,omitempty"`
	Content     []Content `json:"content"`
}

// Toolset executes computer-use tool calls against a Driver.
type Toolset struct {
	driver   Driver
	confirm  func(ctx context.Context, c Call) (bool, error)
	disabled map[string]bool
	legacy   *legacyConfig
}

type legacyConfig struct {
	version       string
	width, height int
}

// New builds the current toolset over d.
func New(d Driver, cfg Config) (*Toolset, error) {
	t := &Toolset{driver: d, confirm: cfg.Confirm, disabled: map[string]bool{}}
	for _, m := range cfg.Disabled {
		t.disabled[m] = true
	}
	for m := range needsConfirm {
		if !t.disabled[m] && t.confirm == nil {
			return nil, ErrConfirmRequired
		}
	}
	return t, nil
}

// Legacy builds a toolset for computer_20241022, computer_20250124 or
// computer_20251124. Those declare a single tool named "computer" with the
// display size and carry an "action" field in each input.
func Legacy(d Driver, version string, widthPx, heightPx int, cfg Config) (*Toolset, error) {
	switch version {
	case "computer_20241022", "computer_20250124", "computer_20251124":
	default:
		return nil, fmt.Errorf("computeruse: unknown legacy version %q", version)
	}
	if widthPx <= 0 || heightPx <= 0 {
		return nil, errors.New("computeruse: display size is required for a legacy tool")
	}
	t, err := New(d, cfg)
	if err != nil {
		return nil, err
	}
	t.legacy = &legacyConfig{version: version, width: widthPx, height: heightPx}
	return t, nil
}

// Tools returns the tool declarations for the Messages API tools array.
func (t *Toolset) Tools() []map[string]interface{} {
	if t.legacy != nil {
		return []map[string]interface{}{{
			"type": t.legacy.version, "name": "computer",
			"display_width_px": t.legacy.width, "display_height_px": t.legacy.height,
		}}
	}
	return []map[string]interface{}{{"type": ToolsetType}}
}

// BetaHeader is the anthropic-beta value a legacy version needs; the current
// toolset needs none.
func (t *Toolset) BetaHeader() string {
	if t.legacy == nil {
		return ""
	}
	return map[string]string{
		"computer_20241022": "computer-use-2024-10-22",
		"computer_20250124": "computer-use-2025-01-24",
		"computer_20251124": "computer-use-2025-11-24",
	}[t.legacy.version]
}

func (t *Toolset) result(use ToolUse, isErr bool, content ...Content) ToolResult {
	r := ToolResult{Type: "tool_result", ToolUseID: use.ID, IsError: isErr, Content: content}
	if t.legacy == nil {
		r.ToolsetName = ToolsetName
	}
	return r
}

func (t *Toolset) errorResult(use ToolUse, msg string) ToolResult {
	if len(msg) > maxErrorText {
		msg = msg[:maxErrorText]
	}
	return t.result(use, true, Content{Type: "text", Text: msg})
}

// Execute runs one tool_use block. It never returns an error: failures become
// results with is_error set, as the protocol requires.
func (t *Toolset) Execute(ctx context.Context, use ToolUse) ToolResult {
	member := use.Name
	var params miosa.ActionParams
	if len(use.Input) > 0 {
		if err := json.Unmarshal(use.Input, &params); err != nil {
			return t.errorResult(use, "invalid input: "+err.Error())
		}
	}
	if t.legacy != nil || member == ToolsetName {
		var wrapper struct {
			Action string `json:"action"`
		}
		_ = json.Unmarshal(use.Input, &wrapper)
		member = wrapper.Action
	}
	if member == "" {
		return t.errorResult(use, "no action named")
	}
	known := false
	for _, m := range Members {
		if m == member {
			known = true
		}
	}
	if !known {
		return t.errorResult(use, fmt.Sprintf("unknown action %q", member))
	}
	if t.disabled[member] {
		return t.errorResult(use, fmt.Sprintf("action %q is disabled", member))
	}
	if needsConfirm[member] && t.confirm != nil {
		ok, err := t.confirm(ctx, Call{Member: member, Input: params})
		if err != nil {
			return t.errorResult(use, "confirmation failed: "+err.Error())
		}
		if !ok {
			return t.errorResult(use, fmt.Sprintf("action %q was not approved", member))
		}
	}
	res, err := t.driver.Act(ctx, member, params)
	if err != nil {
		return t.errorResult(use, err.Error())
	}
	switch member {
	case "screenshot", "zoom":
		return t.result(use, false, Content{Type: "image", Source: &ImageSource{
			Type: "base64", MediaType: "image/png", Data: res.ImageBase64}})
	case "cursor_position":
		if res.X == nil || res.Y == nil {
			return t.errorResult(use, "cursor position unavailable")
		}
		return t.result(use, false, Content{Type: "text", Text: fmt.Sprintf("X=%d, Y=%d", *res.X, *res.Y)})
	}
	return t.result(use, false, Content{Type: "text", Text: "OK"})
}

// ExecuteBatch runs the blocks in order and stops at the first failure: earlier
// results are normal, the failed one has is_error, and every later one carries
// SkippedText.
func (t *Toolset) ExecuteBatch(ctx context.Context, uses []ToolUse) []ToolResult {
	out := make([]ToolResult, 0, len(uses))
	failed := false
	for _, use := range uses {
		if failed {
			out = append(out, t.errorResult(use, SkippedText))
			continue
		}
		r := t.Execute(ctx, use)
		if r.IsError {
			failed = true
		}
		out = append(out, r)
	}
	return out
}

// Describe is a one-line summary for logs.
func (c Call) Describe() string {
	switch {
	case c.Input.Text != "":
		return c.Member + " " + strings.TrimSpace(c.Input.Text)
	case len(c.Input.Coordinate) == 2:
		return fmt.Sprintf("%s at %d,%d", c.Member, c.Input.Coordinate[0], c.Input.Coordinate[1])
	}
	return c.Member
}
