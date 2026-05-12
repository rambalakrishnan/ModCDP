package transport

import (
	"fmt"
	"net"
	"reflect"

	"github.com/pirate/ModCDP/go/modcdp/injector"
	"github.com/pirate/ModCDP/go/modcdp/launcher"
	"github.com/pirate/ModCDP/go/modcdp/types"
)

type ExtensionInjectorConfig = types.ExtensionInjectorConfig
type LaunchOptions = types.LaunchOptions

const DefaultModCDPExtensionID = injector.DefaultModCDPExtensionID

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func boolPtr(value bool) *bool {
	return &value
}

func freePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func websocketURLFor(endpoint string) (string, error) {
	return launcher.WebsocketURLFor(endpoint)
}

type UpstreamMode string
type UpstreamEndpointKind string

const (
	UpstreamModeWS              UpstreamMode = "ws"
	UpstreamModePipe            UpstreamMode = "pipe"
	UpstreamModeNativeMessaging UpstreamMode = "nativemessaging"
	UpstreamModeReverseWS       UpstreamMode = "reversews"
	UpstreamModeNATS            UpstreamMode = "nats"

	UpstreamEndpointKindRawCDP       UpstreamEndpointKind = "raw_cdp"
	UpstreamEndpointKindModCDPServer UpstreamEndpointKind = "modcdp_server"
)

type UpstreamTransport struct {
	recvListeners  []func(map[string]any)
	closeListeners []func(error)
}

func (e *UpstreamTransport) Update(config map[string]any) {
}

func (e *UpstreamTransport) Connect() error {
	return fmt.Errorf("%T.Connect is not implemented", e)
}

func (e *UpstreamTransport) Close() error {
	return nil
}

func (e *UpstreamTransport) Send(message map[string]any) error {
	return fmt.Errorf("%T.Send is not implemented", e)
}

func (e *UpstreamTransport) GetInjectorConfig() ExtensionInjectorConfig {
	return ExtensionInjectorConfig{}
}

func (e *UpstreamTransport) GetLauncherConfig() LaunchOptions {
	return LaunchOptions{}
}

func (e *UpstreamTransport) GetServerConfig() map[string]any {
	return map[string]any{}
}

func (e *UpstreamTransport) OnRecv(listener func(map[string]any)) func() {
	e.recvListeners = append(e.recvListeners, listener)
	return func() {
		pointer := reflect.ValueOf(listener).Pointer()
		for index, candidate := range e.recvListeners {
			if reflect.ValueOf(candidate).Pointer() == pointer {
				e.recvListeners = append(e.recvListeners[:index], e.recvListeners[index+1:]...)
				return
			}
		}
	}
}

func (e *UpstreamTransport) OnClose(listener func(error)) func() {
	e.closeListeners = append(e.closeListeners, listener)
	return func() {
		pointer := reflect.ValueOf(listener).Pointer()
		for index, candidate := range e.closeListeners {
			if reflect.ValueOf(candidate).Pointer() == pointer {
				e.closeListeners = append(e.closeListeners[:index], e.closeListeners[index+1:]...)
				return
			}
		}
	}
}

func (e *UpstreamTransport) emitRecv(message map[string]any) {
	for _, listener := range e.recvListeners {
		listener(message)
	}
}

func (e *UpstreamTransport) EmitRecv(message map[string]any) {
	e.emitRecv(message)
}

func (e *UpstreamTransport) emitClose(err error) {
	for _, listener := range e.closeListeners {
		listener(err)
	}
}

func (e *UpstreamTransport) EmitClose(err error) {
	e.emitClose(err)
}

func (e *UpstreamTransport) WaitForPeer() error {
	return nil
}

func (e *UpstreamTransport) PeerGeneration() int64 {
	return 0
}

func EndpointKindForUpstream(mode string) UpstreamEndpointKind {
	if mode == "ws" || mode == "pipe" {
		return UpstreamEndpointKindRawCDP
	}
	return UpstreamEndpointKindModCDPServer
}

func intFromConfig(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case float32:
		return int(typed), true
	default:
		return 0, false
	}
}
