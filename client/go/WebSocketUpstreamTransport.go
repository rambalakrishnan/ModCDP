package modcdp

import (
	"context"
	"encoding/json"
	"net"
	"sync"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

type WebSocketUpstreamTransport struct {
	UpstreamTransport
	URL     string
	Conn    net.Conn
	writeMu sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewWebSocketUpstreamTransport(url string) *WebSocketUpstreamTransport {
	return &WebSocketUpstreamTransport{URL: url}
}

func (t *WebSocketUpstreamTransport) Connect() error {
	t.ctx, t.cancel = context.WithCancel(context.Background())
	conn, _, _, err := ws.Dial(t.ctx, t.URL)
	if err != nil {
		return err
	}
	t.Conn = conn
	return nil
}

func (t *WebSocketUpstreamTransport) Send(message map[string]any) error {
	body, err := json.Marshal(message)
	if err != nil {
		return err
	}
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	return wsutil.WriteClientText(t.Conn, body)
}

func (t *WebSocketUpstreamTransport) Close() error {
	if t.cancel != nil {
		t.cancel()
	}
	if t.Conn != nil {
		return t.Conn.Close()
	}
	return nil
}
