package miosa_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	miosa "github.com/Miosa-osa/miosa-go/v2"
)

func tinyPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// envdDesktop answers like the guest agent: mutating actions {"status":"ok"}.
func envdDesktop(t *testing.T) (*miosa.Computer, *recorder) {
	t.Helper()
	shot := tinyPNG(t, 8, 6)
	client, rec := newHandlerClient(t, func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case p == "/computers/c1":
			writeJSON(w, 200, map[string]any{"id": "c1", "status": "running"})
		case strings.HasSuffix(p, "/desktop/screenshot"), strings.HasSuffix(p, "/desktop/screenshot/region"):
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(shot)
		case strings.HasSuffix(p, "/desktop/cursor"):
			writeJSON(w, 200, map[string]any{"x": 10, "y": 20})
		case strings.HasSuffix(p, "/desktop/screen-size"):
			writeJSON(w, 200, map[string]any{"width": 1280, "height": 720})
		case strings.HasSuffix(p, "/desktop/environment"):
			writeJSON(w, 200, map[string]any{"name": "xfce", "session_type": "x11", "resolution": "1280x720", "width": 1280, "height": 720, "display": ":0"})
		case strings.HasSuffix(p, "/desktop/windows"):
			writeJSON(w, 200, map[string]any{"windows": []map[string]any{{"id": "w1", "name": "Terminal", "x": 1, "y": 2, "width": 3, "height": 4}}})
		case strings.HasSuffix(p, "/desktop/accessibility-tree"):
			writeJSON(w, 200, map[string]any{"role": "root", "children": []any{}})
		case strings.HasSuffix(p, "/desktop/clipboard") && r.Method == "GET":
			writeJSON(w, 200, map[string]any{"text": "copied"})
		case strings.HasSuffix(p, "/size"):
			writeJSON(w, 200, map[string]any{"width": 300, "height": 200})
		case strings.HasSuffix(p, "/position"):
			writeJSON(w, 200, map[string]any{"x": 5, "y": 6})
		case strings.HasSuffix(p, "/desktop/wait"):
			writeJSON(w, 200, map[string]any{"success": true, "waited_seconds": 1})
		default:
			writeJSON(w, 200, map[string]any{"status": "ok"})
		}
	})
	c, err := client.Computers.Get(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}
	return c, rec
}

func TestDesktopActionsAcceptAgentStatusOK(t *testing.T) {
	c, rec := envdDesktop(t)
	if err := c.Click(context.Background(), 1, 2); err != nil {
		t.Fatalf("an envd {status:ok} answer must count as success: %v", err)
	}
	assertReq(t, rec.Last(t), "POST", "/computers/c1/desktop/click")
}

func TestDesktopActionFailureShapes(t *testing.T) {
	for name, body := range map[string]map[string]any{
		"success false": {"success": false, "message": "no display"},
		"error body":    {"error": "xdotool failed"},
		"status failed": {"status": "failed"},
	} {
		t.Run(name, func(t *testing.T) {
			client, _ := newFixedClient(t, 200, body)
			c, _ := client.Computers.Get(context.Background(), "c1")
			if err := c.Key(context.Background(), "Return"); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestDesktopWindowsAndLaunch(t *testing.T) {
	c, rec := envdDesktop(t)
	ws, err := c.Windows(context.Background())
	if err != nil || len(ws) != 1 || ws[0].Name != "Terminal" || ws[0].Title != "Terminal" || ws[0].Width != 3 {
		t.Fatalf("windows = %+v, %v", ws, err)
	}
	if err := c.Launch(context.Background(), "firefox"); err != nil {
		t.Fatal(err)
	}
	b := rec.Last(t).Body(t)
	if b["app"] != "firefox" {
		t.Fatalf("launch body = %v, envd reads app", b)
	}
	var w miosa.WindowInfo
	if err := json.Unmarshal([]byte(`{"id":"x","title":"T","focused":true}`), &w); err != nil || !w.IsFocused || w.Name != "T" {
		t.Fatalf("legacy window decode = %+v, %v", w, err)
	}
}

func TestDesktopFullSurface(t *testing.T) {
	ctx := context.Background()
	c, rec := envdDesktop(t)
	steps := []struct {
		name         string
		run          func() error
		method, path string
		body         map[string]any
	}{
		{"hotkey", func() error { return c.Hotkey(ctx, "ctrl", "c") }, "POST", "/computers/c1/desktop/hotkey", map[string]any{"keys": []any{"ctrl", "c"}}},
		{"keydown", func() error { return c.KeyDown(ctx, "shift") }, "POST", "/computers/c1/desktop/key-down", map[string]any{"key": "shift"}},
		{"keyup", func() error { return c.KeyUp(ctx, "shift") }, "POST", "/computers/c1/desktop/key-up", map[string]any{"key": "shift"}},
		{"move", func() error { return c.MouseMove(ctx, 3, 4) }, "POST", "/computers/c1/desktop/move", map[string]any{"x": 3.0, "y": 4.0}},
		{"mousedown", func() error { return c.MouseDown(ctx, 3, 4, miosa.ButtonRight) }, "POST", "/computers/c1/desktop/mouse-down", map[string]any{"x": 3.0, "y": 4.0, "button": "right"}},
		{"mouseup", func() error { return c.MouseUp(ctx, 3, 4) }, "POST", "/computers/c1/desktop/mouse-up", map[string]any{"x": 3.0, "y": 4.0}},
		{"setclipboard", func() error { return c.SetClipboard(ctx, "hi") }, "POST", "/computers/c1/desktop/clipboard", map[string]any{"text": "hi"}},
		{"wallpaper", func() error { return c.SetWallpaper(ctx, "/tmp/a.png") }, "POST", "/computers/c1/desktop/wallpaper", map[string]any{"path": "/tmp/a.png"}},
		{"resize", func() error { return c.ResizeWindow(ctx, "w 1", 800, 600) }, "POST", "/computers/c1/desktop/window/w 1/resize", map[string]any{"width": 800.0, "height": 600.0}},
		{"movewindow", func() error { return c.MoveWindow(ctx, "w1", 7, 8) }, "POST", "/computers/c1/desktop/window/w1/move", map[string]any{"x": 7.0, "y": 8.0}},
		{"maximize", func() error { return c.MaximizeWindow(ctx, "w1") }, "POST", "/computers/c1/desktop/window/w1/maximize", nil},
		{"minimize", func() error { return c.MinimizeWindow(ctx, "w1") }, "POST", "/computers/c1/desktop/window/w1/minimize", nil},
		{"close", func() error { return c.CloseWindow(ctx, "w1") }, "POST", "/computers/c1/desktop/window/w1/close", nil},
	}
	for _, s := range steps {
		if err := s.run(); err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		got := rec.Last(t)
		if got.Method != s.method || got.Path != s.path {
			t.Fatalf("%s: %s %s, want %s %s", s.name, got.Method, got.Path, s.method, s.path)
		}
		for k, v := range s.body {
			if gb := got.Body(t); gb[k] == nil || toJSON(gb[k]) != toJSON(v) {
				t.Fatalf("%s: body %v, want %s=%v", s.name, gb, k, v)
			}
		}
	}
	if err := c.Hotkey(ctx); err == nil {
		t.Fatal("empty hotkey must fail")
	}

	size, err := c.ScreenSize(ctx)
	if err != nil || size.Width != 1280 || size.Height != 720 {
		t.Fatalf("size = %+v, %v", size, err)
	}
	env, err := c.DesktopInfo(ctx)
	if err != nil || env.Name != "xfce" || env.SessionType != "x11" {
		t.Fatalf("env = %+v, %v", env, err)
	}
	if text, err := c.Clipboard(ctx); err != nil || text != "copied" {
		t.Fatalf("clipboard = %q, %v", text, err)
	}
	if tree, err := c.AccessibilityTree(ctx); err != nil || !strings.Contains(string(tree), `"root"`) {
		t.Fatalf("tree = %s, %v", tree, err)
	}
	if ws, err := c.WindowSize(ctx, "w1"); err != nil || ws.Width != 300 {
		t.Fatalf("window size = %+v, %v", ws, err)
	}
	if wp, err := c.WindowPosition(ctx, "w1"); err != nil || wp.X != 5 {
		t.Fatalf("window position = %+v, %v", wp, err)
	}
}

func toJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

func TestDesktopLook(t *testing.T) {
	c, rec := envdDesktop(t)
	st, err := c.Look(context.Background(), miosa.LookOptions{})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.StdEncoding.DecodeString(st.ImageBase64)
	if len(raw) == 0 || st.Width != 8 || st.Height != 6 || !strings.Contains(string(st.Tree), "root") {
		t.Fatalf("state = %+v", st)
	}
	before := rec.Count()
	st, err = c.Look(context.Background(), miosa.LookOptions{NoImage: true})
	if err != nil || st.ImageBase64 != "" || rec.Count() != before+1 {
		t.Fatalf("tree-only look = %+v, %v, requests +%d", st, err, rec.Count()-before)
	}
}

func TestDesktopAct(t *testing.T) {
	ctx := context.Background()
	c, rec := envdDesktop(t)
	last := func() recorded { return rec.Last(t) }

	shot, err := c.Act(ctx, "screenshot", miosa.ActionParams{})
	if err != nil || !shot.OK || shot.Width != 8 || shot.ImageBase64 == "" || shot.Action != "screenshot" {
		t.Fatalf("screenshot = %+v, %v", shot, err)
	}
	cur, err := c.Act(ctx, "cursor_position", miosa.ActionParams{})
	if err != nil || *cur.X != 10 || *cur.Y != 20 {
		t.Fatalf("cursor = %+v, %v", cur, err)
	}
	if res, err := c.Act(ctx, "left_click", miosa.ActionParams{Coordinate: []int{5, 6}}); err != nil || *res.X != 5 {
		t.Fatalf("left_click = %+v, %v", res, err)
	}
	if b := last().Body(t); b["x"].(float64) != 5 || b["button"] != nil && b["button"] != "left" {
		t.Fatalf("click body = %v", b)
	}
	// No coordinate: clicks where the cursor is.
	if _, err := c.Act(ctx, "right_click", miosa.ActionParams{}); err != nil {
		t.Fatal(err)
	}
	if b := last().Body(t); b["x"].(float64) != 10 || b["button"] != "right" {
		t.Fatalf("right_click body = %v", b)
	}
	// Modifier held around the click.
	before := rec.Count()
	if _, err := c.Act(ctx, "left_click", miosa.ActionParams{Coordinate: []int{1, 1}, Text: "shift"}); err != nil {
		t.Fatal(err)
	}
	var seq []string
	for _, r := range rec.reqs[before:] {
		seq = append(seq, r.Path[strings.LastIndex(r.Path, "/")+1:])
	}
	if strings.Join(seq, ",") != "key-down,click,key-up" {
		t.Fatalf("modifier sequence = %v", seq)
	}
	if _, err := c.Act(ctx, "key", miosa.ActionParams{Text: "ctrl+s"}); err != nil {
		t.Fatal(err)
	}
	assertReq(t, last(), "POST", "/computers/c1/desktop/hotkey")
	before = rec.Count()
	if _, err := c.Act(ctx, "key", miosa.ActionParams{Text: "Tab", Repeat: 3}); err != nil || rec.Count()-before != 3 {
		t.Fatalf("repeat sent %d requests, %v", rec.Count()-before, err)
	}
	if _, err := c.Act(ctx, "scroll", miosa.ActionParams{ScrollDirection: "down", ScrollAmount: 2, Coordinate: []int{4, 4}}); err != nil {
		t.Fatal(err)
	}
	if b := last().Body(t); b["direction"] != "down" || b["clicks"].(float64) != 2 {
		t.Fatalf("scroll body = %v", b)
	}
	if _, err := c.Act(ctx, "zoom", miosa.ActionParams{Region: []int{10, 20, 110, 70}}); err != nil {
		t.Fatal(err)
	}
	if b := last().Body(t); b["width"].(float64) != 100 || b["height"].(float64) != 50 {
		t.Fatalf("zoom body = %v", b)
	}
	if _, err := c.Act(ctx, "hold_key", miosa.ActionParams{Text: "shift", Duration: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Act(ctx, "left_click_drag", miosa.ActionParams{StartCoordinate: []int{1, 2}, Coordinate: []int{3, 4}}); err != nil {
		t.Fatal(err)
	}
	if b := last().Body(t); b["from_x"].(float64) != 1 || b["to_y"].(float64) != 4 {
		t.Fatalf("drag body = %v", b)
	}

	for name, p := range map[string]miosa.ActionParams{
		"scroll":          {ScrollDirection: "sideways"},
		"left_click_drag": {Coordinate: []int{1, 2}},
		"mouse_move":      {},
		"zoom":            {Region: []int{1, 2}},
		"type":            {},
		"wait":            {Duration: 301},
		"key":             {Text: "a", Repeat: 101},
	} {
		if res, err := c.Act(ctx, name, p); err == nil || res == nil || res.OK {
			t.Errorf("%s with %+v: res=%+v err=%v, want a failed result", name, p, res, err)
		}
	}
	if _, err := c.Act(ctx, "teleport", miosa.ActionParams{}); err == nil || !strings.Contains(err.Error(), "unknown desktop action") {
		t.Fatalf("unknown action err = %v", err)
	}
	for _, a := range miosa.Actions {
		if a == "" {
			t.Fatal("empty action name")
		}
	}
}

func TestComputerWaiters(t *testing.T) {
	var gets, probes int32
	client, _ := newHandlerClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/computers/c1":
			status := "starting"
			if atomic.AddInt32(&gets, 1) >= 3 {
				status = "running"
			}
			writeJSON(w, 200, map[string]any{"id": "c1", "status": status})
		case strings.HasSuffix(r.URL.Path, "/screen-size"):
			if atomic.AddInt32(&probes, 1) < 3 {
				writeJSON(w, 502, map[string]any{"error": "agent_unavailable"})
				return
			}
			writeJSON(w, 200, map[string]any{"width": 1, "height": 1})
		}
	})
	ctx := context.Background()
	c := &miosa.Computer{}
	got, err := client.Computers.Get(ctx, "c1")
	if err != nil {
		t.Fatal(err)
	}
	c = got
	if err := c.WaitForDesktop(ctx, miosa.WaitOptions{Poll: time.Millisecond, Timeout: 5 * time.Second}); err != nil {
		t.Fatal(err)
	}
	if gets < 3 || probes < 3 {
		t.Fatalf("polled %d refreshes and %d probes", gets, probes)
	}

	failed, _ := newFixedClient(t, 200, map[string]any{"id": "c2", "status": "error"})
	bad, _ := failed.Computers.Get(ctx, "c2")
	if err := bad.WaitUntilReady(ctx, miosa.WaitOptions{}); err == nil || !strings.Contains(err.Error(), "terminal state") {
		t.Fatalf("err = %v", err)
	}

	slow, _ := newFixedClient(t, 200, map[string]any{"id": "c3", "status": "starting"})
	st, _ := slow.Computers.Get(ctx, "c3")
	if err := st.WaitUntilReady(ctx, miosa.WaitOptions{Timeout: 30 * time.Millisecond, Poll: 5 * time.Millisecond}); err == nil {
		t.Fatal("expected a timeout")
	}
}

func TestComputerViewerURL(t *testing.T) {
	client, rec := newHandlerClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/computers/c1" {
			writeJSON(w, 200, map[string]any{"id": "c1", "status": "running"})
			return
		}
		writeJSON(w, 200, map[string]any{"token": "tok", "expires_at": 1800000000, "ttl_seconds": 600, "embed_url": "https://e", "ws_url": "wss://w"})
	})
	c, _ := client.Computers.Get(context.Background(), "c1")
	v, err := c.ViewerURL(context.Background(), 10*time.Minute)
	if err != nil || v.URL != "https://e" || v.Token != "tok" || v.ExpiresAt.Unix() != 1800000000 {
		t.Fatalf("viewer = %+v, %v", v, err)
	}
	if rec.Last(t).RawQuery != "ttl_seconds=600" {
		t.Fatalf("query = %q", rec.Last(t).RawQuery)
	}
}
