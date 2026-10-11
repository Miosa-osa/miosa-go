package computeruse_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	miosa "github.com/Miosa-osa/miosa-go/v2"
	"github.com/Miosa-osa/miosa-go/v2/computeruse"
)

type fakeDriver struct {
	calls []string
	fail  map[string]error
}

func (f *fakeDriver) Act(_ context.Context, action string, p miosa.ActionParams) (*miosa.ActionResult, error) {
	f.calls = append(f.calls, action)
	if err := f.fail[action]; err != nil {
		return nil, err
	}
	x, y := 512, 384
	return &miosa.ActionResult{OK: true, Action: action, X: &x, Y: &y, ImageBase64: "aGk="}, nil
}

func use(id, name, input string) computeruse.ToolUse {
	return computeruse.ToolUse{ID: id, Name: name, Input: json.RawMessage(input)}
}

func approve(context.Context, computeruse.Call) (bool, error) { return true, nil }

func TestToolsDeclaration(t *testing.T) {
	ts, err := computeruse.New(&fakeDriver{}, computeruse.Config{Confirm: approve})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(ts.Tools())
	if string(raw) != `[{"type":"computer_toolset_20260801"}]` {
		t.Fatalf("tools = %s", raw)
	}
	if ts.BetaHeader() != "" {
		t.Fatal("the current toolset needs no beta header")
	}
	if len(computeruse.Members) != 17 {
		t.Fatalf("members = %d, want 17", len(computeruse.Members))
	}
}

func TestConfirmRequiredForKeyboardMembers(t *testing.T) {
	if _, err := computeruse.New(&fakeDriver{}, computeruse.Config{}); !errors.Is(err, computeruse.ErrConfirmRequired) {
		t.Fatalf("err = %v", err)
	}
	if _, err := computeruse.New(&fakeDriver{}, computeruse.Config{Disabled: []string{"type", "key", "hold_key"}}); err != nil {
		t.Fatalf("disabled keyboard members need no confirm: %v", err)
	}
}

func TestResultShapes(t *testing.T) {
	ts, _ := computeruse.New(&fakeDriver{}, computeruse.Config{Confirm: approve})
	ctx := context.Background()

	shot := ts.Execute(ctx, use("t1", "screenshot", `{}`))
	if shot.IsError || shot.ToolsetName != "computer" || shot.ToolUseID != "t1" ||
		shot.Content[0].Type != "image" || shot.Content[0].Source.MediaType != "image/png" || shot.Content[0].Source.Data != "aGk=" {
		t.Fatalf("screenshot = %+v", shot)
	}
	cur := ts.Execute(ctx, use("t2", "cursor_position", `{}`))
	if cur.Content[0].Text != "X=512, Y=384" {
		t.Fatalf("cursor = %+v", cur)
	}
	click := ts.Execute(ctx, use("t3", "left_click", `{"coordinate":[1,2]}`))
	if click.IsError || click.Content[0].Text != "OK" || click.ToolsetName != "computer" {
		t.Fatalf("click = %+v", click)
	}
	raw, _ := json.Marshal(click)
	if !strings.Contains(string(raw), `"toolset_name":"computer"`) {
		t.Fatalf("toolset_name missing: %s", raw)
	}
}

func TestBatchHaltsAtFirstFailure(t *testing.T) {
	d := &fakeDriver{fail: map[string]error{"left_click": errors.New("no such window")}}
	ts, _ := computeruse.New(d, computeruse.Config{Confirm: approve})
	res := ts.ExecuteBatch(context.Background(), []computeruse.ToolUse{
		use("a", "mouse_move", `{"coordinate":[1,1]}`),
		use("b", "left_click", `{"coordinate":[1,1]}`),
		use("c", "type", `{"text":"hi"}`),
		use("d", "screenshot", `{}`),
	})
	if len(res) != 4 || res[0].IsError {
		t.Fatalf("results = %+v", res)
	}
	if !res[1].IsError || res[1].Content[0].Text != "no such window" {
		t.Fatalf("failed result = %+v", res[1])
	}
	for _, r := range res[2:] {
		if !r.IsError || r.Content[0].Text != "Not executed: an earlier computer action in this turn failed." || r.ToolsetName != "computer" {
			t.Fatalf("skipped result = %+v", r)
		}
	}
	if strings.Join(d.calls, ",") != "mouse_move,left_click" {
		t.Fatalf("driver ran %v; later actions must not run", d.calls)
	}
}

func TestConfirmGate(t *testing.T) {
	d := &fakeDriver{}
	var asked []string
	ts, _ := computeruse.New(d, computeruse.Config{Confirm: func(_ context.Context, c computeruse.Call) (bool, error) {
		asked = append(asked, c.Member)
		return c.Input.Text != "forbidden text", nil
	}})
	ctx := context.Background()
	if r := ts.Execute(ctx, use("1", "type", `{"text":"hello"}`)); r.IsError {
		t.Fatalf("approved type failed: %+v", r)
	}
	r := ts.Execute(ctx, use("2", "type", `{"text":"forbidden text"}`))
	if !r.IsError || !strings.Contains(r.Content[0].Text, "not approved") {
		t.Fatalf("denied = %+v", r)
	}
	ts.Execute(ctx, use("3", "left_click", `{"coordinate":[1,1]}`))
	if strings.Join(asked, ",") != "type,type" {
		t.Fatalf("asked for %v; clicks need no confirmation", asked)
	}
	if strings.Join(d.calls, ",") != "type,left_click" {
		t.Fatalf("driver ran %v", d.calls)
	}

	boom, _ := computeruse.New(d, computeruse.Config{Confirm: func(context.Context, computeruse.Call) (bool, error) {
		return false, errors.New("prompt closed")
	}})
	if r := boom.Execute(ctx, use("4", "key", `{"text":"Return"}`)); !r.IsError || !strings.Contains(r.Content[0].Text, "prompt closed") {
		t.Fatalf("confirm error = %+v", r)
	}
}

func TestDisabledUnknownAndBadInput(t *testing.T) {
	ts, _ := computeruse.New(&fakeDriver{}, computeruse.Config{Confirm: approve, Disabled: []string{"zoom"}})
	ctx := context.Background()
	for name, u := range map[string]computeruse.ToolUse{
		"disabled": use("1", "zoom", `{"region":[0,0,1,1]}`),
		"unknown":  use("2", "teleport", `{}`),
		"bad json": use("3", "left_click", `{"coordinate":`),
		"no name":  use("4", "", `{}`),
	} {
		if r := ts.Execute(ctx, u); !r.IsError || r.ToolsetName != "computer" {
			t.Errorf("%s: %+v", name, r)
		}
	}
}

func TestLongErrorsAreCut(t *testing.T) {
	d := &fakeDriver{fail: map[string]error{"screenshot": errors.New(strings.Repeat("x", 10000))}}
	ts, _ := computeruse.New(d, computeruse.Config{Confirm: approve})
	r := ts.Execute(context.Background(), use("1", "screenshot", `{}`))
	if len(r.Content[0].Text) != 4096 {
		t.Fatalf("error text length = %d, want 4096", len(r.Content[0].Text))
	}
}

func TestLegacyTools(t *testing.T) {
	d := &fakeDriver{}
	ts, err := computeruse.Legacy(d, "computer_20251124", 1280, 720, computeruse.Config{Confirm: approve})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(ts.Tools())
	if string(raw) != `[{"display_height_px":720,"display_width_px":1280,"name":"computer","type":"computer_20251124"}]` {
		t.Fatalf("tools = %s", raw)
	}
	if ts.BetaHeader() != "computer-use-2025-11-24" {
		t.Fatalf("beta = %q", ts.BetaHeader())
	}
	r := ts.Execute(context.Background(), use("1", "computer", `{"action":"left_click","coordinate":[3,4]}`))
	if r.IsError || r.ToolsetName != "" {
		t.Fatalf("legacy result = %+v (no toolset_name for legacy tools)", r)
	}
	if strings.Join(d.calls, ",") != "left_click" {
		t.Fatalf("driver ran %v", d.calls)
	}
	if _, err := computeruse.Legacy(d, "computer_1999", 1, 1, computeruse.Config{Confirm: approve}); err == nil {
		t.Fatal("unknown version must fail")
	}
	if _, err := computeruse.Legacy(d, "computer_20250124", 0, 0, computeruse.Config{Confirm: approve}); err == nil {
		t.Fatal("missing display size must fail")
	}
}

// TestAgainstAComputer runs the toolset over a real *miosa.Computer talking to
// an httptest agent, end to end.
func TestAgainstAComputer(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/computers/c1":
			_, _ = w.Write([]byte(`{"id":"c1","status":"running"}`))
		case strings.HasSuffix(r.URL.Path, "/desktop/cursor"):
			_, _ = w.Write([]byte(`{"x":7,"y":9}`))
		default:
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		}
	}))
	defer srv.Close()
	client := miosa.NewClient("k", miosa.WithBaseURL(srv.URL), miosa.WithMaxRetries(0))
	comp, err := client.Computers.Get(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}
	ts, _ := computeruse.New(comp, computeruse.Config{Confirm: approve})
	res := ts.ExecuteBatch(context.Background(), []computeruse.ToolUse{
		use("1", "left_click", `{"coordinate":[10,20]}`),
		use("2", "cursor_position", `{}`),
		use("3", "type", `{"text":"hello"}`),
	})
	for _, r := range res {
		if r.IsError {
			t.Fatalf("result = %+v", r)
		}
	}
	if res[1].Content[0].Text != "X=7, Y=9" {
		t.Fatalf("cursor text = %q", res[1].Content[0].Text)
	}
	want := "POST /computers/c1/desktop/click,GET /computers/c1/desktop/cursor,POST /computers/c1/desktop/type"
	if got := strings.Join(paths[1:], ","); got != want {
		t.Fatalf("requests = %s", got)
	}
}
