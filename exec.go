package miosa

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ExecService runs commands on a computer, supporting both one-shot execution
// and interactive PTY sessions via WebSocket.
// Accessed via Computer.Exec.
type ExecService struct {
	client     *Client
	computerID string
}

func (s *ExecService) base() string {
	return fmt.Sprintf("/computers/%s/exec", s.computerID)
}

// Bash runs a shell command synchronously and returns the combined output.
func (s *ExecService) Bash(ctx context.Context, command string, timeoutSecs ...int) (*ExecResult, error) {
	const op = "ExecService.Bash"
	input := ExecInput{Command: command}
	if len(timeoutSecs) > 0 {
		input.Timeout = timeoutSecs[0]
	}
	var out ExecResult
	if err := s.client.postJSON(ctx, s.base(), input, &out); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &out, nil
}

// Python runs Python code synchronously and returns the combined output.
func (s *ExecService) Python(ctx context.Context, code string, timeoutSecs ...int) (*ExecResult, error) {
	const op = "ExecService.Python"
	input := ExecPythonInput{Code: code}
	if len(timeoutSecs) > 0 {
		input.Timeout = timeoutSecs[0]
	}
	var out ExecResult
	if err := s.client.postJSON(ctx, s.base()+"/python", input, &out); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &out, nil
}

// SpawnOptions configures an interactive exec session.
type SpawnOptions struct {
	// Rows is the initial terminal height in lines (default 24).
	Rows uint16
	// Cols is the initial terminal width in columns (default 80).
	Cols uint16
	// Env holds additional environment variables for the spawned process.
	Env map[string]string
	// Args are passed to the command.
	Args []string
	// Cwd is the working directory.
	Cwd string
	// NoTTY runs the command without a PTY (separate stdout and stderr).
	// The default allocates a PTY, where output arrives on Stdout.
	NoTTY bool
}

// execProtocol is the WebSocket subprotocol of the framed exec stream.
const execProtocol = "miosa-exec-v1"

// Frame types of the miosa-exec-v1 protocol: one type byte, then the payload.
const (
	frameStdin     = 0x00 // client to server
	frameStdout    = 0x01
	frameStderr    = 0x02
	frameExit      = 0x03 // big-endian u32 exit code
	frameStdinEOF  = 0x04 // client to server, no payload
	frameResize    = 0x05 // client to server: rows u16, cols u16 (big-endian)
	framePortOpen  = 0x10 // JSON payload
	framePortClose = 0x11 // JSON payload
)

// Spawn opens an interactive exec session for command over WebSocket
// (WS /computers/:id/exec/stream, subprotocol miosa-exec-v1). Returns a *Cmd
// that mirrors the os/exec.Cmd shape: read Stdout/Stderr, write to Stdin,
// resize the terminal, and wait for exit. Closing Stdin sends end-of-input.
//
//	cmd, err := computer.Exec.Spawn(ctx, "bash", SpawnOptions{Rows: 40, Cols: 120})
//	if err != nil { ... }
//	defer cmd.Kill()
//	io.WriteString(cmd.Stdin, "ls -la\n")
//	exit, _ := cmd.Wait()
func (s *ExecService) Spawn(ctx context.Context, command string, opts SpawnOptions) (*Cmd, error) {
	const op = "ExecService.Spawn"

	wsURL := buildExecWSURL(s.client.baseURL, s.computerID, command, opts)
	header := http.Header{}
	s.client.setAuthHeaders(ctx, header)

	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 30 * time.Second,
		Subprotocols:     []string{execProtocol},
	}
	conn, _, err := dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()

	cmd := &Cmd{
		Stdin:  stdinW,
		Stdout: stdoutR,
		Stderr: stderrR,
		conn:   conn,
		done:   make(chan int, 1),
	}

	// stdin pump: frame what the caller writes; end of input becomes stdin_eof.
	go func() {
		defer stdinR.Close()
		buf := make([]byte, 4096)
		for {
			n, err := stdinR.Read(buf)
			if n > 0 {
				frame := append([]byte{frameStdin}, buf[:n]...)
				if werr := cmd.write(frame); werr != nil {
					return
				}
			}
			if err != nil {
				_ = cmd.write([]byte{frameStdinEOF})
				return
			}
		}
	}()

	// read pump: demux frames.
	go func() {
		defer stdoutW.Close()
		defer stderrW.Close()
		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				cmd.finish(-1)
				return
			}
			if mt != websocket.BinaryMessage || len(msg) == 0 {
				continue
			}
			payload := msg[1:]
			switch msg[0] {
			case frameStdout:
				_, _ = stdoutW.Write(payload)
			case frameStderr:
				_, _ = stderrW.Write(payload)
			case frameExit:
				code := 0
				if len(payload) >= 4 {
					code = int(int32(binary.BigEndian.Uint32(payload[:4])))
				}
				cmd.finish(code)
				return
			case framePortOpen, framePortClose:
				if cmd.TextMessageHandler != nil {
					cmd.TextMessageHandler(payload)
				}
			}
		}
	}()

	return cmd, nil
}

// buildExecWSURL constructs the WebSocket URL for an interactive session.
func buildExecWSURL(baseURL, computerID, command string, opts SpawnOptions) string {
	wsBase := strings.Replace(baseURL, "https://", "wss://", 1)
	wsBase = strings.Replace(wsBase, "http://", "ws://", 1)

	rows := opts.Rows
	if rows == 0 {
		rows = 24
	}
	cols := opts.Cols
	if cols == 0 {
		cols = 80
	}

	q := url.Values{}
	q.Set("command", command)
	q.Set("tty", strconv.FormatBool(!opts.NoTTY))
	q.Set("rows", strconv.Itoa(int(rows)))
	q.Set("cols", strconv.Itoa(int(cols)))
	if len(opts.Args) > 0 {
		q.Set("args", strings.Join(opts.Args, " "))
	}
	if opts.Cwd != "" {
		q.Set("cwd", opts.Cwd)
	}
	keys := make([]string, 0, len(opts.Env))
	for k := range opts.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		q.Add("env", k+"="+opts.Env[k])
	}
	return fmt.Sprintf("%s/computers/%s/exec/stream?%s", wsBase, url.PathEscape(computerID), q.Encode())
}

// ─── Cmd ─────────────────────────────────────────────────────────────────────

// Cmd represents a running interactive PTY session.
// It mirrors the shape of os/exec.Cmd with WebSocket-backed I/O.
type Cmd struct {
	// Stdin writes data to the PTY's stdin.
	Stdin io.Writer
	// Stdout reads data from the PTY's stdout.
	Stdout io.Reader
	// Stderr reads data from the PTY's stderr.
	Stderr io.Reader
	// TextMessageHandler, if set, is called with the JSON payload of each
	// port_opened and port_closed frame.
	TextMessageHandler func([]byte)

	conn     *websocket.Conn
	wmu      sync.Mutex
	done     chan int
	doneOnce sync.Once
	code     int
}

// Wait blocks until the remote process exits and returns its exit code.
// Returns -1 if the connection was lost before an exit code was received.
func (c *Cmd) Wait() (int, error) {
	c.doneOnce.Do(func() { c.code = <-c.done })
	return c.code, nil
}

func (c *Cmd) finish(code int) {
	select {
	case c.done <- code:
	default:
	}
}

func (c *Cmd) write(frame []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	return c.conn.WriteMessage(websocket.BinaryMessage, frame)
}

// Resize sends a terminal resize event to the PTY.
func (c *Cmd) Resize(rows, cols uint16) error {
	frame := make([]byte, 5)
	frame[0] = frameResize
	binary.BigEndian.PutUint16(frame[1:3], rows)
	binary.BigEndian.PutUint16(frame[3:5], cols)
	if err := c.write(frame); err != nil {
		return fmt.Errorf("Cmd.Resize: %w", err)
	}
	return nil
}

// Kill closes the WebSocket connection, terminating the remote process.
func (c *Cmd) Kill() error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	return c.conn.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "killed"))
}
