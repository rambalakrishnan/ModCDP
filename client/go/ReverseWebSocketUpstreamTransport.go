package modcdp

type ReverseWebSocketUpstreamTransport struct {
	UpstreamTransport
	Bind string
}

func NewReverseWebSocketUpstreamTransport(bind string) *ReverseWebSocketUpstreamTransport {
	return &ReverseWebSocketUpstreamTransport{Bind: bind}
}

func (t *ReverseWebSocketUpstreamTransport) Connect() error {
	return unimplementedUpstream("reversews")
}

func (t *ReverseWebSocketUpstreamTransport) Send(message map[string]any) error {
	return unimplementedUpstream("reversews")
}

func (t *ReverseWebSocketUpstreamTransport) Close() error {
	return nil
}
