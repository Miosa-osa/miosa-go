package miosa_test

import (
	"context"
	"encoding/binary"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	miosa "github.com/Miosa-osa/miosa-go/v2"
	"github.com/gorilla/websocket"
)

// wsExecServer speaks the server side of miosa-exec-v1 for one connection.
type wsExecServer struct {
	mu       sync.Mutex
	path     string
	query    url.Values
	protocol string
	auth     string
	frames   [][]byte // client frames, in order
	reply    func(conn *websocket.Conn, frame []byte) bool
	done     chan struct{}
}

func newWSExec(t *testing.T, reply func(conn *websocket.Conn, frame []byte) bool) (*miosa.Computer, *wsExecServer) {
	t.Helper()
	srv := &wsExecServer{reply: reply, done: make(chan struct{})}
	upgrader := websocket.Upgrader{Subprotocols: []string{"miosa-exec-v1"}}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/computers/cmp_1" {
			writeJSON(w, 200, map[string]interface{}{"id": "cmp_1", "status": "running"})
			return
		}
		srv.mu.Lock()
		srv.path = r.URL.Path
		srv.query = r.URL.Query()
		srv.auth = r.Header.Get("Authorization")
		srv.mu.Unlock()
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		srv.mu.Lock()
		srv.protocol = conn.Subprotocol()
		srv.mu.Unlock()
		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				close(srv.done)
				return
			}
			if mt != websocket.BinaryMessage {
				continue
			}
			srv.mu.Lock()
			srv.frames = append(srv.frames, append([]byte(nil), msg...))
			srv.mu.Unlock()
			if srv.reply != nil && !srv.reply(conn, msg) {
				close(srv.done)
				return
			}
		}
	}))
	t.Cleanup(ts.Close)
	client := miosa.NewClient("msk_u_ws", miosa.WithBaseURL(ts.URL), miosa.WithMaxRetries(0))
	c, err := client.Computers.Get(context.Background(), "cmp_1")
	if err != nil {
		t.Fatal(err)
	}
	return c, srv
}

func exitFrame(code int32) []byte {
	f := make([]byte, 5)
	f[0] = 0x03
	binary.BigEndian.PutUint32(f[1:], uint32(code))
	return f
}

func TestSpawnSpeaksTheFramedExecProtocol(t *testing.T) {
	comp, srv := newWSExec(t, func(conn *websocket.Conn, frame []byte) bool {
		if frame[0] == 0x00 { // stdin: echo as stdout, complain on stderr, then exit 7
			_ = conn.WriteMessage(websocket.BinaryMessage, append([]byte{0x01}, frame[1:]...))
			_ = conn.WriteMessage(websocket.BinaryMessage, append([]byte{0x02}, []byte("warn")...))
			_ = conn.WriteMessage(websocket.BinaryMessage, exitFrame(7))
		}
		return true
	})

	cmd, err := comp.Exec.Spawn(context.Background(), "bash", miosa.SpawnOptions{
		Rows: 40, Cols: 120, Args: []string{"-lc", "ls"}, Cwd: "/work", Env: map[string]string{"B": "2", "A": "1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cmd.Kill()

	if _, err := io.WriteString(cmd.Stdin, "hello"); err != nil {
		t.Fatal(err)
	}
	out := make([]byte, 5)
	if _, err := io.ReadFull(cmd.Stdout, out); err != nil || string(out) != "hello" {
		t.Fatalf("stdout = %q, %v", out, err)
	}
	errOut := make([]byte, 4)
	if _, err := io.ReadFull(cmd.Stderr, errOut); err != nil || string(errOut) != "warn" {
		t.Fatalf("stderr = %q, %v", errOut, err)
	}
	code, _ := cmd.Wait()
	if code != 7 {
		t.Errorf("exit code = %d", code)
	}

	srv.mu.Lock()
	defer srv.mu.Unlock()
	if srv.path != "/computers/cmp_1/exec/stream" {
		t.Errorf("path = %q; the route is /exec/stream, not /exec/spawn", srv.path)
	}
	if srv.protocol != "miosa-exec-v1" {
		t.Errorf("subprotocol = %q", srv.protocol)
	}
	if srv.auth != "Bearer msk_u_ws" {
		t.Errorf("Authorization = %q", srv.auth)
	}
	q := srv.query
	if q.Get("command") != "bash" || q.Get("tty") != "true" || q.Get("rows") != "40" || q.Get("cols") != "120" ||
		q.Get("args") != "-lc ls" || q.Get("cwd") != "/work" {
		t.Errorf("query = %v", q)
	}
	if env := q["env"]; len(env) != 2 || env[0] != "A=1" || env[1] != "B=2" {
		t.Errorf("env = %v", env)
	}
}

func TestSpawnNoTTYAndDefaults(t *testing.T) {
	comp, srv := newWSExec(t, nil)
	cmd, err := comp.Exec.Spawn(context.Background(), "echo hi & more", miosa.SpawnOptions{NoTTY: true})
	if err != nil {
		t.Fatal(err)
	}
	defer cmd.Kill()
	// Give the server a moment to record the handshake.
	time.Sleep(50 * time.Millisecond)
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if srv.query.Get("tty") != "false" || srv.query.Get("rows") != "24" || srv.query.Get("cols") != "80" || srv.query.Get("command") != "echo hi & more" {
		t.Errorf("query = %v (the command must survive URL encoding)", srv.query)
	}
}

func TestSpawnResizeAndStdinEOF(t *testing.T) {
	comp, srv := newWSExec(t, nil)
	cmd, err := comp.Exec.Spawn(context.Background(), "cat", miosa.SpawnOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer cmd.Kill()

	if err := cmd.Resize(50, 200); err != nil {
		t.Fatal(err)
	}
	if c, ok := cmd.Stdin.(io.Closer); ok {
		_ = c.Close()
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		srv.mu.Lock()
		n := len(srv.frames)
		srv.mu.Unlock()
		if n >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if len(srv.frames) < 2 {
		t.Fatalf("frames = %v", srv.frames)
	}
	resize := srv.frames[0]
	if len(resize) != 5 || resize[0] != 0x05 || binary.BigEndian.Uint16(resize[1:3]) != 50 || binary.BigEndian.Uint16(resize[3:5]) != 200 {
		t.Errorf("resize frame = %v", resize)
	}
	if eof := srv.frames[1]; len(eof) != 1 || eof[0] != 0x04 {
		t.Errorf("closing Stdin must send stdin_eof, got %v", eof)
	}
}

func TestSpawnPortFramesReachTheHandler(t *testing.T) {
	comp, _ := newWSExec(t, func(conn *websocket.Conn, frame []byte) bool {
		_ = conn.WriteMessage(websocket.BinaryMessage, append([]byte{0x10}, []byte(`{"port":3000}`)...))
		_ = conn.WriteMessage(websocket.BinaryMessage, exitFrame(0))
		return true
	})
	cmd, err := comp.Exec.Spawn(context.Background(), "npm start", miosa.SpawnOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer cmd.Kill()
	got := make(chan string, 1)
	cmd.TextMessageHandler = func(b []byte) {
		select {
		case got <- string(b):
		default:
		}
	}
	_, _ = io.WriteString(cmd.Stdin, "go")
	select {
	case msg := <-got:
		if msg != `{"port":3000}` {
			t.Errorf("port payload = %q", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no port frame reached the handler")
	}
	if code, _ := cmd.Wait(); code != 0 {
		t.Errorf("exit code = %d", code)
	}
}

func TestSpawnConnectionLossIsExitMinusOne(t *testing.T) {
	comp, _ := newWSExec(t, func(conn *websocket.Conn, frame []byte) bool {
		conn.Close() // drop without an exit frame
		return false
	})
	cmd, err := comp.Exec.Spawn(context.Background(), "x", miosa.SpawnOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(cmd.Stdin, "go")
	code, _ := cmd.Wait()
	if code != -1 {
		t.Errorf("exit code = %d, want -1", code)
	}
	if code2, _ := cmd.Wait(); code2 != -1 {
		t.Errorf("a second Wait must return the same code, got %d", code2)
	}
}

func TestSpawnRefusedHandshakeIsAnError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/computers/cmp_1" {
			writeJSON(w, 200, map[string]interface{}{"id": "cmp_1"})
			return
		}
		http.Error(w, "no", http.StatusForbidden)
	}))
	defer ts.Close()
	client := miosa.NewClient("k", miosa.WithBaseURL(ts.URL))
	c, _ := client.Computers.Get(context.Background(), "cmp_1")
	if _, err := c.Exec.Spawn(context.Background(), "x", miosa.SpawnOptions{}); err == nil {
		t.Fatal("a refused upgrade must be an error")
	}
}
