package miosa

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Wire contract: envd /desktop/* behind /computers/:id/desktop/*. Mutating
// actions answer {"status":"ok"}; reads answer the documents below.

// ScreenSize is the display size in pixels.
type ScreenSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// DesktopEnvironment describes the desktop session.
type DesktopEnvironment struct {
	Name        string `json:"name"`
	SessionType string `json:"session_type"`
	Resolution  string `json:"resolution"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Display     string `json:"display"`
}

// WindowSize and WindowPosition are the geometry of one window.
type WindowSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// WindowPosition is a window's top-left corner.
type WindowPosition struct {
	X int `json:"x"`
	Y int `json:"y"`
}

func (c *Computer) desktopPath(rest string) string {
	return "/computers/" + url.PathEscape(c.ID) + "/desktop/" + rest
}

func (c *Computer) desktopGet(ctx context.Context, rest string, out interface{}) error {
	return c.client.getJSON(ctx, c.desktopPath(rest), out)
}

// ─── Keyboard ────────────────────────────────────────────────────────────────

// Hotkey presses keys together and releases them, for example Hotkey("ctrl", "c").
func (c *Computer) Hotkey(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return errors.New("miosa: hotkey needs at least one key")
	}
	return c.desktopPost(ctx, "hotkey", map[string]interface{}{"keys": keys})
}

// KeyDown holds a key until KeyUp.
func (c *Computer) KeyDown(ctx context.Context, key string) error {
	return c.desktopPost(ctx, "key-down", map[string]string{"key": key})
}

// KeyUp releases a key held with KeyDown.
func (c *Computer) KeyUp(ctx context.Context, key string) error {
	return c.desktopPost(ctx, "key-up", map[string]string{"key": key})
}

// ─── Mouse ───────────────────────────────────────────────────────────────────

// MouseMove moves the cursor without clicking.
func (c *Computer) MouseMove(ctx context.Context, x, y int) error {
	return c.desktopPost(ctx, "move", map[string]int{"x": x, "y": y})
}

// MouseDown presses a mouse button at (x, y) until MouseUp.
// The API gateway publishes /desktop/mouse-down, but envd serves
// /desktop/mouse-press, so this can answer 404 until the gateway maps it.
func (c *Computer) MouseDown(ctx context.Context, x, y int, button ...MouseButton) error {
	return c.desktopPost(ctx, "mouse-down", mouseBody(x, y, button))
}

// MouseUp releases a mouse button. See MouseDown for the routing caveat.
func (c *Computer) MouseUp(ctx context.Context, x, y int, button ...MouseButton) error {
	return c.desktopPost(ctx, "mouse-up", mouseBody(x, y, button))
}

func mouseBody(x, y int, button []MouseButton) map[string]interface{} {
	b := map[string]interface{}{"x": x, "y": y}
	if len(button) > 0 && button[0] != "" {
		b["button"] = string(button[0])
	}
	return b
}

// ─── Display, clipboard, environment ─────────────────────────────────────────

// ScreenSize returns the display size.
func (c *Computer) ScreenSize(ctx context.Context) (*ScreenSize, error) {
	var out ScreenSize
	if err := c.desktopGet(ctx, "screen-size", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DesktopInfo describes the desktop session. (Computer.Environment is the
// environment the machine was created from.)
func (c *Computer) DesktopInfo(ctx context.Context) (*DesktopEnvironment, error) {
	var out DesktopEnvironment
	if err := c.desktopGet(ctx, "environment", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Clipboard returns the clipboard text.
func (c *Computer) Clipboard(ctx context.Context) (string, error) {
	var out struct {
		Text string `json:"text"`
	}
	if err := c.desktopGet(ctx, "clipboard", &out); err != nil {
		return "", err
	}
	return out.Text, nil
}

// SetClipboard replaces the clipboard text.
func (c *Computer) SetClipboard(ctx context.Context, text string) error {
	return c.desktopPost(ctx, "clipboard", map[string]string{"text": text})
}

// SetWallpaper sets the wallpaper from a path inside the computer or a URL.
func (c *Computer) SetWallpaper(ctx context.Context, pathOrURL string) error {
	return c.desktopPost(ctx, "wallpaper", map[string]string{"path": pathOrURL})
}

// AccessibilityTree returns the UI element tree as the agent reports it
// (role, name, bounds, children). Treat it as opaque JSON.
func (c *Computer) AccessibilityTree(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	if err := c.desktopGet(ctx, "accessibility-tree", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ReapplyBranding re-applies the tenant's desktop branding.
func (c *Computer) ReapplyBranding(ctx context.Context) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := c.client.postJSON(ctx, c.desktopPath("branding/reapply"), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ─── Windows ─────────────────────────────────────────────────────────────────

func (c *Computer) windowPath(id, rest string) string {
	return "window/" + url.PathEscape(id) + "/" + rest
}

// WindowSize returns a window's size.
func (c *Computer) WindowSize(ctx context.Context, windowID string) (*WindowSize, error) {
	var out WindowSize
	if err := c.desktopGet(ctx, c.windowPath(windowID, "size"), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// WindowPosition returns a window's position.
func (c *Computer) WindowPosition(ctx context.Context, windowID string) (*WindowPosition, error) {
	var out WindowPosition
	if err := c.desktopGet(ctx, c.windowPath(windowID, "position"), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ResizeWindow sets a window's size.
func (c *Computer) ResizeWindow(ctx context.Context, windowID string, width, height int) error {
	return c.desktopPost(ctx, c.windowPath(windowID, "resize"), map[string]int{"width": width, "height": height})
}

// MoveWindow sets a window's position.
func (c *Computer) MoveWindow(ctx context.Context, windowID string, x, y int) error {
	return c.desktopPost(ctx, c.windowPath(windowID, "move"), map[string]int{"x": x, "y": y})
}

// MaximizeWindow maximizes a window.
func (c *Computer) MaximizeWindow(ctx context.Context, windowID string) error {
	return c.desktopPost(ctx, c.windowPath(windowID, "maximize"), map[string]string{})
}

// MinimizeWindow minimizes a window.
func (c *Computer) MinimizeWindow(ctx context.Context, windowID string) error {
	return c.desktopPost(ctx, c.windowPath(windowID, "minimize"), map[string]string{})
}

// CloseWindow closes a window.
func (c *Computer) CloseWindow(ctx context.Context, windowID string) error {
	return c.desktopPost(ctx, c.windowPath(windowID, "close"), map[string]string{})
}

// ─── Look: screenshot and tree in one call ───────────────────────────────────

// LookOptions picks what Look returns; both default to on.
type LookOptions struct {
	NoImage bool
	NoTree  bool
}

// WindowState is what the desktop looks like right now.
type WindowState struct {
	ImageBase64 string          `json:"image_base64,omitempty"`
	Width       int             `json:"width,omitempty"`
	Height      int             `json:"height,omitempty"`
	Tree        json.RawMessage `json:"tree,omitempty"`
}

// Look fetches the screenshot and the accessibility tree together. The two
// requests run in parallel; until the API has a one-round-trip route this costs
// the slower of the two.
func (c *Computer) Look(ctx context.Context, opts LookOptions) (*WindowState, error) {
	var (
		wg       sync.WaitGroup
		out      WindowState
		imgErr   error
		treeErr  error
		pngBytes []byte
	)
	if !opts.NoImage {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pngBytes, imgErr = c.Screenshot(ctx)
		}()
	}
	if !opts.NoTree {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out.Tree, treeErr = c.AccessibilityTree(ctx)
		}()
	}
	wg.Wait()
	if imgErr != nil {
		return nil, imgErr
	}
	if treeErr != nil {
		return nil, treeErr
	}
	if len(pngBytes) > 0 {
		out.ImageBase64 = base64.StdEncoding.EncodeToString(pngBytes)
		if cfg, err := png.DecodeConfig(bytes.NewReader(pngBytes)); err == nil {
			out.Width, out.Height = cfg.Width, cfg.Height
		}
	}
	return &out, nil
}

// ─── Typed actions ───────────────────────────────────────────────────────────

// ActionParams are the inputs of Act. They use the member names Anthropic's
// computer-use toolset sends, so a model's tool_use input decodes into it.
type ActionParams struct {
	// Coordinate is [x, y]; StartCoordinate is the drag origin.
	Coordinate      []int `json:"coordinate,omitempty"`
	StartCoordinate []int `json:"start_coordinate,omitempty"`
	// Region is [x0, y0, x1, y1] for zoom.
	Region []int `json:"region,omitempty"`
	// Text is the string to type, a key or "+"-joined combination, or the
	// modifier keys held during a click or scroll.
	Text            string  `json:"text,omitempty"`
	ScrollDirection string  `json:"scroll_direction,omitempty"`
	ScrollAmount    int     `json:"scroll_amount,omitempty"`
	Duration        float64 `json:"duration,omitempty"`
	Repeat          int     `json:"repeat,omitempty"`
}

// ActionResult says what an action did. Image actions fill ImageBase64.
type ActionResult struct {
	OK       bool          `json:"ok"`
	Action   string        `json:"action"`
	X        *int          `json:"x,omitempty"`
	Y        *int          `json:"y,omitempty"`
	Width    int           `json:"width,omitempty"`
	Height   int           `json:"height,omitempty"`
	Status   string        `json:"status,omitempty"`
	Message  string        `json:"message,omitempty"`
	Duration time.Duration `json:"duration_ns"`
	// ImageBase64 is a PNG for screenshot and zoom.
	ImageBase64 string `json:"image_base64,omitempty"`
}

// ErrUnknownAction is returned by Act for a name it does not know.
var ErrUnknownAction = errors.New("miosa: unknown desktop action")

// Actions lists the names Act accepts.
var Actions = []string{
	"screenshot", "zoom", "left_click", "right_click", "middle_click", "double_click", "triple_click",
	"left_click_drag", "mouse_move", "left_mouse_down", "left_mouse_up", "cursor_position",
	"scroll", "type", "key", "hold_key", "wait",
}

func point(v []int, name string) (int, int, error) {
	if len(v) != 2 {
		return 0, 0, fmt.Errorf("miosa: %s must be [x, y]", name)
	}
	return v[0], v[1], nil
}

func modifiers(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(text, "+")
}

// withModifiers holds the keys in text around fn.
func (c *Computer) withModifiers(ctx context.Context, text string, fn func() error) error {
	mods := modifiers(text)
	for i, m := range mods {
		if err := c.KeyDown(ctx, m); err != nil {
			for _, held := range mods[:i] {
				_ = c.KeyUp(ctx, held)
			}
			return err
		}
	}
	err := fn()
	for i := len(mods) - 1; i >= 0; i-- {
		if uerr := c.KeyUp(ctx, mods[i]); uerr != nil && err == nil {
			err = uerr
		}
	}
	return err
}

// at resolves the click point: the coordinate, or where the cursor is.
func (c *Computer) at(ctx context.Context, p ActionParams) (int, int, error) {
	if len(p.Coordinate) == 0 {
		cur, err := c.Cursor(ctx)
		if err != nil {
			return 0, 0, err
		}
		return cur.X, cur.Y, nil
	}
	return point(p.Coordinate, "coordinate")
}

// Act runs one desktop action by name and returns a typed result. It is the
// single entry point an agent loop needs; the individual methods stay.
func (c *Computer) Act(ctx context.Context, action string, p ActionParams) (*ActionResult, error) {
	start := time.Now()
	res := &ActionResult{Action: action}
	finish := func(err error) (*ActionResult, error) {
		res.Duration = time.Since(start)
		if err != nil {
			return res, err
		}
		res.OK = true
		return res, nil
	}

	switch action {
	case "screenshot":
		b, err := c.Screenshot(ctx)
		if err == nil {
			res.ImageBase64 = base64.StdEncoding.EncodeToString(b)
			if cfg, derr := png.DecodeConfig(bytes.NewReader(b)); derr == nil {
				res.Width, res.Height = cfg.Width, cfg.Height
			}
		}
		return finish(err)
	case "zoom":
		if len(p.Region) != 4 {
			return finish(errors.New("miosa: region must be [x0, y0, x1, y1]"))
		}
		b, err := c.ScreenshotRegion(ctx, p.Region[0], p.Region[1], p.Region[2]-p.Region[0], p.Region[3]-p.Region[1])
		if err == nil {
			res.ImageBase64 = base64.StdEncoding.EncodeToString(b)
			if cfg, derr := png.DecodeConfig(bytes.NewReader(b)); derr == nil {
				res.Width, res.Height = cfg.Width, cfg.Height
			}
		}
		return finish(err)
	case "left_click", "right_click", "middle_click", "double_click", "triple_click":
		x, y, err := c.at(ctx, p)
		if err != nil {
			return finish(err)
		}
		res.X, res.Y = &x, &y
		return finish(c.withModifiers(ctx, p.Text, func() error {
			switch action {
			case "right_click":
				return c.Click(ctx, x, y, ButtonRight)
			case "middle_click":
				return c.Click(ctx, x, y, ButtonMiddle)
			case "double_click":
				return c.DoubleClick(ctx, x, y)
			case "triple_click":
				if err := c.DoubleClick(ctx, x, y); err != nil {
					return err
				}
				return c.Click(ctx, x, y)
			}
			return c.Click(ctx, x, y)
		}))
	case "left_click_drag":
		fx, fy, err := point(p.StartCoordinate, "start_coordinate")
		if err != nil {
			return finish(err)
		}
		tx, ty, err := point(p.Coordinate, "coordinate")
		if err != nil {
			return finish(err)
		}
		res.X, res.Y = &tx, &ty
		return finish(c.withModifiers(ctx, p.Text, func() error { return c.Drag(ctx, fx, fy, tx, ty) }))
	case "mouse_move":
		x, y, err := point(p.Coordinate, "coordinate")
		if err != nil {
			return finish(err)
		}
		res.X, res.Y = &x, &y
		return finish(c.MouseMove(ctx, x, y))
	case "left_mouse_down", "left_mouse_up":
		x, y, err := c.at(ctx, ActionParams{})
		if err != nil {
			return finish(err)
		}
		res.X, res.Y = &x, &y
		if action == "left_mouse_down" {
			return finish(c.MouseDown(ctx, x, y, ButtonLeft))
		}
		return finish(c.MouseUp(ctx, x, y, ButtonLeft))
	case "cursor_position":
		cur, err := c.Cursor(ctx)
		if err == nil {
			res.X, res.Y = &cur.X, &cur.Y
		}
		return finish(err)
	case "scroll":
		dir := ScrollDirection(p.ScrollDirection)
		switch dir {
		case ScrollUp, ScrollDown, ScrollLeft, ScrollRight:
		default:
			return finish(fmt.Errorf("miosa: scroll_direction %q is not up, down, left or right", p.ScrollDirection))
		}
		amount := p.ScrollAmount
		if amount <= 0 {
			amount = 3
		}
		return finish(c.withModifiers(ctx, p.Text, func() error {
			if len(p.Coordinate) == 2 {
				res.X, res.Y = &p.Coordinate[0], &p.Coordinate[1]
				return c.ScrollAt(ctx, p.Coordinate[0], p.Coordinate[1], dir, amount)
			}
			return c.Scroll(ctx, dir, amount)
		}))
	case "type":
		if p.Text == "" {
			return finish(errors.New("miosa: text is required"))
		}
		return finish(c.Type(ctx, p.Text))
	case "key":
		if p.Text == "" {
			return finish(errors.New("miosa: text is required"))
		}
		n := p.Repeat
		if n <= 0 {
			n = 1
		}
		if n > 100 {
			return finish(errors.New("miosa: repeat is at most 100"))
		}
		keys := strings.Split(p.Text, "+")
		for i := 0; i < n; i++ {
			var err error
			if len(keys) > 1 {
				err = c.Hotkey(ctx, keys...)
			} else {
				err = c.Key(ctx, p.Text)
			}
			if err != nil {
				return finish(err)
			}
		}
		return finish(nil)
	case "hold_key":
		if p.Text == "" || p.Duration <= 0 || p.Duration > 300 {
			return finish(errors.New("miosa: hold_key needs text and a duration up to 300 seconds"))
		}
		keys := strings.Split(p.Text, "+")
		for i, k := range keys {
			if err := c.KeyDown(ctx, k); err != nil {
				for _, held := range keys[:i] {
					_ = c.KeyUp(ctx, held)
				}
				return finish(err)
			}
		}
		werr := c.DesktopWait(ctx, p.Duration)
		for i := len(keys) - 1; i >= 0; i-- {
			if uerr := c.KeyUp(ctx, keys[i]); uerr != nil && werr == nil {
				werr = uerr
			}
		}
		return finish(werr)
	case "wait":
		if p.Duration <= 0 || p.Duration > 300 {
			return finish(errors.New("miosa: wait needs a duration up to 300 seconds"))
		}
		return finish(c.DesktopWait(ctx, p.Duration))
	}
	return nil, fmt.Errorf("%w: %q", ErrUnknownAction, action)
}

// ─── Waiters and viewer ──────────────────────────────────────────────────────

// WaitOptions tune the Computer waiters.
type WaitOptions struct {
	// Timeout bounds the whole wait. Default 120 s.
	Timeout time.Duration
	// Poll is the pause between checks. Default 1 s.
	Poll time.Duration
}

func (o WaitOptions) timeout() time.Duration {
	if o.Timeout <= 0 {
		return 120 * time.Second
	}
	return o.Timeout
}

func (o WaitOptions) poll() time.Duration {
	if o.Poll <= 0 {
		return time.Second
	}
	return o.Poll
}

// ErrComputerFailed is returned by the waiters when the computer reached a
// state it will not leave on its own.
var ErrComputerFailed = errors.New("miosa: computer is in a terminal state")

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// WaitUntilReady polls until the computer is running (or active). It returns
// ErrComputerFailed for error, stopped or destroyed, and the context error on
// timeout.
func (c *Computer) WaitUntilReady(ctx context.Context, opts WaitOptions) error {
	ctx, cancel := context.WithTimeout(ctx, opts.timeout())
	defer cancel()
	for {
		switch c.Status {
		case StatusRunning, StatusActive:
			return nil
		case StatusError, StatusDestroyed, StatusStopped:
			return fmt.Errorf("%w: %s", ErrComputerFailed, c.Status)
		}
		if err := sleepCtx(ctx, opts.poll()); err != nil {
			return fmt.Errorf("miosa: computer %s not ready (status %s): %w", c.ID, c.Status, err)
		}
		if err := c.Refresh(ctx); err != nil {
			return err
		}
	}
}

// WaitForDesktop waits until the computer is ready and its desktop answers.
func (c *Computer) WaitForDesktop(ctx context.Context, opts WaitOptions) error {
	if err := c.WaitUntilReady(ctx, opts); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, opts.timeout())
	defer cancel()
	for {
		_, err := c.ScreenSize(ctx)
		if err == nil {
			return nil
		}
		if cerr := ctx.Err(); cerr != nil {
			return fmt.Errorf("miosa: desktop of %s not answering: %w (last error: %v)", c.ID, cerr, err)
		}
		if serr := sleepCtx(ctx, opts.poll()); serr != nil {
			return fmt.Errorf("miosa: desktop of %s not answering: %w (last error: %v)", c.ID, serr, err)
		}
	}
}

// Viewer is a typed embed link.
type Viewer struct {
	URL       string
	ExpiresAt time.Time
	// Token is the short-lived stream token the URL carries.
	Token string
	// WSURL is the websockify endpoint for custom viewers.
	WSURL string
}

// ViewerURL mints an embeddable viewer link valid for ttl (zero uses the
// server default). The API does not yet offer a view-only or clipboard switch
// on this route, so the link grants full control while it lives; keep ttl short
// for untrusted viewers, or hand them CreateViewerSession tokens instead.
func (c *Computer) ViewerURL(ctx context.Context, ttl time.Duration) (*Viewer, error) {
	access, err := c.client.Computers.Embed(ctx, c.ID, int(ttl/time.Second))
	if err != nil {
		return nil, err
	}
	return &Viewer{
		URL:       access.EmbedURL,
		ExpiresAt: time.Unix(access.ExpiresAt, 0).UTC(),
		Token:     access.Token,
		WSURL:     access.WSURL,
	}, nil
}
