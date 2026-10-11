package computeruse_test

import (
	"context"
	"encoding/json"

	miosa "github.com/Miosa-osa/miosa-go/v2"
	"github.com/Miosa-osa/miosa-go/v2/computeruse"
)

// Serve Claude's computer-use tool calls from a MIOSA desktop.
func ExampleToolset_ExecuteBatch() {
	ctx := context.Background()
	comp, _ := miosa.NewClient("msk_u_...").Computers.Get(ctx, "cmp_123")

	ts, err := computeruse.New(comp, computeruse.Config{
		// Required while type, key or hold_key are enabled.
		Confirm: func(ctx context.Context, c computeruse.Call) (bool, error) { return true, nil },
	})
	if err != nil {
		return
	}
	_ = ts.Tools() // [{"type":"computer_toolset_20260801"}] for the Messages API tools array

	// Each tool_use block from the model's reply, in order.
	uses := []computeruse.ToolUse{
		{ID: "toolu_1", Name: "left_click", Input: json.RawMessage(`{"coordinate":[512,384]}`)},
		{ID: "toolu_2", Name: "screenshot", Input: json.RawMessage(`{}`)},
	}
	results := ts.ExecuteBatch(ctx, uses) // goes back as tool_result blocks in the next user message
	_ = results
}
