package modcdp

type NativeMessagingUpstreamTransport struct {
	UpstreamTransport
	ManifestPath string
	HostName     string
}

func NewNativeMessagingUpstreamTransport(manifestPath string) *NativeMessagingUpstreamTransport {
	return &NativeMessagingUpstreamTransport{ManifestPath: manifestPath, HostName: "com.modcdp.bridge"}
}

func (t *NativeMessagingUpstreamTransport) Connect() error {
	return unimplementedUpstream("nativemessaging")
}

func (t *NativeMessagingUpstreamTransport) Send(message map[string]any) error {
	return unimplementedUpstream("nativemessaging")
}

func (t *NativeMessagingUpstreamTransport) Close() error {
	return nil
}
