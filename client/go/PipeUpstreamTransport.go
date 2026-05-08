package modcdp

type PipeUpstreamTransport struct {
	UpstreamTransport
}

func NewPipeUpstreamTransport() *PipeUpstreamTransport {
	return &PipeUpstreamTransport{}
}

func (t *PipeUpstreamTransport) Connect() error {
	return unimplementedUpstream("pipe")
}

func (t *PipeUpstreamTransport) Send(message map[string]any) error {
	return unimplementedUpstream("pipe")
}

func (t *PipeUpstreamTransport) Close() error {
	return nil
}
