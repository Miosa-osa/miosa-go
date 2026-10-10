package miosa

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// DialWebSocket opens a WebSocket to a path under the API base URL (for
// example "/sandboxes/sbx_1/ssh-tunnel"), authenticated as this client: bearer
// token, tenant, user agent and default headers, plus any request headers on
// ctx. Optional subprotocols are offered in the handshake.
//
// It is the building block for the raw byte tunnels (ssh-tunnel, port
// tunnels) and the framed exec protocol. The caller owns the returned
// connection and must close it.
func (c *Client) DialWebSocket(ctx context.Context, path string, subprotocols ...string) (*websocket.Conn, *http.Response, error) {
	wsURL := strings.Replace(c.baseURL, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1) + path
	header := http.Header{}
	c.setAuthHeaders(ctx, header)
	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 30 * time.Second,
		Subprotocols:     subprotocols,
	}
	conn, resp, err := dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return nil, resp, err
	}
	return conn, resp, nil
}
