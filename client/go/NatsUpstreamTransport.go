package modcdp

type NatsUpstreamTransport struct {
	UpstreamTransport
	URL string
}

func NewNatsUpstreamTransport(url string) *NatsUpstreamTransport {
	return &NatsUpstreamTransport{URL: url}
}

func (t *NatsUpstreamTransport) Connect() error {
	return unimplementedUpstream("nats")
}

func (t *NatsUpstreamTransport) Send(message map[string]any) error {
	return unimplementedUpstream("nats")
}

func (t *NatsUpstreamTransport) Close() error {
	return nil
}
