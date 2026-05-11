// ModCDPClient (Go): importable, no CLI, no demo code.
//
// Option groups mirror the JS / Python ports:
//
//	Launch         browser/session creation and cleanup.
//	Upstream       message transport to raw CDP or a ModCDP server.
//	Extension      raw-CDP extension discovery/injection/borrowing.
//	Client.Routes  client-side direct_cdp/service_worker routing.
//	Server         ModCDPServer.configure params.
//	Client        client routing and client-owned send/event timings.
//	Extension     extension discovery, wake, probe, and keepalive timings.
//	Upstream      upstream transport options and upstream-owned timings.
//
// Public methods: Connect, Send(method, params), SendRaw, On, OnRaw, Close.
// Synchronous; one background goroutine reads messages off the WS.
//
// Route and ModCDP wire translation lives in translate.go. Launchers and
// upstream transports live in their matching class files.
package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	abxjsonschema "github.com/ArchiveBox/abxbus/abxbus-go/jsonschema"
	"github.com/pirate/ModCDP/go/modcdp/injector"
	"github.com/pirate/ModCDP/go/modcdp/launcher"
	"github.com/pirate/ModCDP/go/modcdp/router"
	"github.com/pirate/ModCDP/go/modcdp/translate"
	transportpkg "github.com/pirate/ModCDP/go/modcdp/transport"
	"github.com/pirate/ModCDP/go/modcdp/types"
)

var (
	extIDFromURL = regexp.MustCompile(`^chrome-extension://([a-z]+)/`)
)

const modcdpReadyExpression = `Boolean(globalThis.ModCDP?.__ModCDPServerVersion === 1 && globalThis.ModCDP?.handleCommand && globalThis.ModCDP?.addCustomEvent)`

const DefaultCDPSendTimeoutMS = 10_000
const DefaultEventWaitTimeoutMS = 10_000
const DefaultExecutionContextTimeoutMS = 10_000
const DefaultChromeReadyTimeoutMS = 45_000
const DefaultChromeReadyPollIntervalMS = 100
const DefaultServiceWorkerProbeTimeoutMS = 10_000
const DefaultServiceWorkerReadyTimeoutMS = 60_000
const DefaultServiceWorkerPollIntervalMS = 100
const DefaultTargetSessionPollIntervalMS = 20
const DefaultWSConnectErrorSettleTimeoutMS = 250

type LaunchOptions = types.LaunchOptions
type LaunchedBrowser = launcher.LaunchedBrowser
type BrowserLauncher = launcher.BrowserLauncher
type LocalBrowserLauncher = launcher.LocalBrowserLauncher
type RemoteBrowserLauncher = launcher.RemoteBrowserLauncher
type BrowserbaseBrowserLauncher = launcher.BrowserbaseBrowserLauncher
type NoopBrowserLauncher = launcher.NoopBrowserLauncher
type ExtensionInjectorConfig = types.ExtensionInjectorConfig
type ExtensionInjectionResult = types.ExtensionInjectionResult
type SendCDP = types.SendCDP
type AttachToTarget = types.AttachToTarget
type ExtensionInjector = injector.ExtensionInjector
type DiscoveredExtensionInjector = injector.DiscoveredExtensionInjector
type BBBrowserExtensionInjector = injector.BBBrowserExtensionInjector
type LocalBrowserLaunchExtensionInjector = injector.LocalBrowserLaunchExtensionInjector
type ExtensionsLoadUnpackedInjector = injector.ExtensionsLoadUnpackedInjector
type BorrowedExtensionInjector = injector.BorrowedExtensionInjector
type UpstreamMode = transportpkg.UpstreamMode
type UpstreamEndpointKind = transportpkg.UpstreamEndpointKind
type UpstreamTransport = transportpkg.UpstreamTransport
type WebSocketUpstreamTransport = transportpkg.WebSocketUpstreamTransport
type WebSocketUpstreamTransportOptions = transportpkg.WebSocketUpstreamTransportOptions
type PipeUpstreamTransport = transportpkg.PipeUpstreamTransport
type PipeUpstreamTransportOptions = transportpkg.PipeUpstreamTransportOptions
type ReverseWebSocketUpstreamTransport = transportpkg.ReverseWebSocketUpstreamTransport
type ReverseWebSocketUpstreamTransportOptions = transportpkg.ReverseWebSocketUpstreamTransportOptions
type NativeMessagingUpstreamTransport = transportpkg.NativeMessagingUpstreamTransport
type NativeMessagingUpstreamTransportOptions = transportpkg.NativeMessagingUpstreamTransportOptions
type NatsUpstreamTransport = transportpkg.NatsUpstreamTransport
type NatsUpstreamTransportOptions = transportpkg.NatsUpstreamTransportOptions
type AutoSessionRouter = router.AutoSessionRouter

var NewLocalBrowserLauncher = launcher.NewLocalBrowserLauncher
var NewRemoteBrowserLauncher = launcher.NewRemoteBrowserLauncher
var NewBrowserbaseBrowserLauncher = launcher.NewBrowserbaseBrowserLauncher
var NewNoopBrowserLauncher = launcher.NewNoopBrowserLauncher
var NewDiscoveredExtensionInjector = injector.NewDiscoveredExtensionInjector
var NewBBBrowserExtensionInjector = injector.NewBBBrowserExtensionInjector
var NewLocalBrowserLaunchExtensionInjector = injector.NewLocalBrowserLaunchExtensionInjector
var NewExtensionsLoadUnpackedInjector = injector.NewExtensionsLoadUnpackedInjector
var NewBorrowedExtensionInjector = injector.NewBorrowedExtensionInjector
var NewWebSocketUpstreamTransport = transportpkg.NewWebSocketUpstreamTransport
var NewPipeUpstreamTransport = transportpkg.NewPipeUpstreamTransport
var NewReverseWebSocketUpstreamTransport = transportpkg.NewReverseWebSocketUpstreamTransport
var NewNativeMessagingUpstreamTransport = transportpkg.NewNativeMessagingUpstreamTransport
var NewNatsUpstreamTransport = transportpkg.NewNatsUpstreamTransport
var NewAutoSessionRouter = router.NewAutoSessionRouter

var DefaultModCDPServiceWorkerURLSuffixes = injector.DefaultModCDPServiceWorkerURLSuffixes

const DefaultModCDPWakePath = injector.DefaultModCDPWakePath
const DefaultModCDPExtensionID = injector.DefaultModCDPExtensionID
const DefaultUpstreamReverseWSBind = transportpkg.DefaultUpstreamReverseWSBind
const DefaultUpstreamReverseWSWaitTimeoutMS = transportpkg.DefaultUpstreamReverseWSWaitTimeoutMS
const DefaultUpstreamNATSWaitTimeoutMS = transportpkg.DefaultUpstreamNATSWaitTimeoutMS
const DefaultUpstreamNativeMessagingWaitTimeoutMS = transportpkg.DefaultUpstreamNativeMessagingWaitTimeoutMS
const UpstreamEndpointKindRawCDP = transportpkg.UpstreamEndpointKindRawCDP
const UpstreamEndpointKindModCDPServer = transportpkg.UpstreamEndpointKindModCDPServer

var endpointKindForUpstream = transportpkg.EndpointKindForUpstream

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func websocketURLFor(endpoint string) (string, error) {
	if strings.HasPrefix(endpoint, "ws://") || strings.HasPrefix(endpoint, "wss://") {
		return endpoint, nil
	}
	httpEndpoint := endpoint
	if !strings.Contains(endpoint, "://") {
		httpEndpoint = "http://" + endpoint
	}
	resp, err := http.Get(httpEndpoint + "/json/version")
	if err != nil {
		return "", fmt.Errorf("GET /json/version: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		parsed, parseErr := url.Parse(httpEndpoint)
		if parseErr != nil {
			return "", parseErr
		}
		if parsed.Scheme == "https" {
			parsed.Scheme = "wss"
		} else {
			parsed.Scheme = "ws"
		}
		parsed.Path = "/devtools/browser"
		parsed.RawQuery = ""
		parsed.Fragment = ""
		return parsed.String(), nil
	}
	body, _ := io.ReadAll(resp.Body)
	var version map[string]any
	if err := json.Unmarshal(body, &version); err != nil {
		return "", fmt.Errorf("parse /json/version: %w", err)
	}
	wsURL, _ := version["webSocketDebuggerUrl"].(string)
	if wsURL == "" {
		return "", fmt.Errorf("HTTP discovery for %s returned no webSocketDebuggerUrl", endpoint)
	}
	return wsURL, nil
}

func freePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

// --- public types --------------------------------------------------------

type ServerConfig struct {
	LoopbackCDPURL                    string            `json:"loopback_cdp_url,omitempty"`
	Routes                            map[string]string `json:"routes,omitempty"`
	BrowserToken                      string            `json:"browser_token,omitempty"`
	CDPSendTimeoutMS                  int               `json:"cdp_send_timeout_ms,omitempty"`
	LoopbackExecutionContextTimeoutMS int               `json:"loopback_execution_context_timeout_ms,omitempty"`
	WSConnectErrorSettleTimeoutMS     int               `json:"ws_connect_error_settle_timeout_ms,omitempty"`
	Options                           map[string]any    `json:"-"`
	disabled                          bool
}

var ServerNone = &ServerConfig{disabled: true}

type CustomEvent struct {
	Name        string         `json:"name"`
	EventSchema map[string]any `json:"event_schema,omitempty"`
}

type CustomCommand struct {
	Name         string         `json:"name"`
	Expression   string         `json:"expression,omitempty"`
	ParamsSchema map[string]any `json:"params_schema,omitempty"`
	ResultSchema map[string]any `json:"result_schema,omitempty"`
}

type CustomMiddleware struct {
	Name       string `json:"name,omitempty"`
	Phase      string `json:"phase"`
	Expression string `json:"expression"`
}

type LaunchConfig struct {
	Mode           string        `json:"mode,omitempty"`
	ExecutablePath string        `json:"executable_path,omitempty"`
	UserDataDir    string        `json:"user_data_dir,omitempty"`
	Options        LaunchOptions `json:"options,omitempty"`
}

type UpstreamConfig struct {
	Mode                                 string `json:"mode,omitempty"`
	CDPURL                               string `json:"cdp_url,omitempty"`
	UpstreamNATSURL                      string `json:"upstream_nats_url,omitempty"`
	UpstreamNATSSubjectPrefix            string `json:"upstream_nats_subject_prefix,omitempty"`
	UpstreamNATSWaitTimeoutMS            int    `json:"upstream_nats_wait_timeout_ms,omitempty"`
	UpstreamReverseWSBind                string `json:"upstream_reversews_bind,omitempty"`
	UpstreamReverseWSWaitTimeoutMS       int    `json:"upstream_reversews_wait_timeout_ms,omitempty"`
	UpstreamNativeMessagingManifest      string `json:"upstream_nativemessaging_manifest,omitempty"`
	UpstreamNativeMessagingHostName      string `json:"upstream_nativemessaging_host_name,omitempty"`
	UpstreamNativeMessagingWaitTimeoutMS int    `json:"upstream_nativemessaging_wait_timeout_ms,omitempty"`
	WSConnectErrorSettleTimeoutMS        int    `json:"ws_connect_error_settle_timeout_ms,omitempty"`
}

type ExtensionConfig struct {
	Mode                         string   `json:"mode,omitempty"`
	Path                         string   `json:"path,omitempty"`
	ExtensionID                  string   `json:"extension_id,omitempty"`
	WakePath                     string   `json:"wake_path,omitempty"`
	WakeURL                      string   `json:"wake_url,omitempty"`
	ServiceWorkerURLIncludes     []string `json:"service_worker_url_includes,omitempty"`
	ServiceWorkerURLSuffixes     []string `json:"service_worker_url_suffixes,omitempty"`
	TrustServiceWorkerTarget     bool     `json:"trust_service_worker_target,omitempty"`
	RequireServiceWorkerTarget   bool     `json:"require_service_worker_target,omitempty"`
	ServiceWorkerReadyExpression string   `json:"service_worker_ready_expression,omitempty"`
	ExecutionContextTimeoutMS    int      `json:"execution_context_timeout_ms,omitempty"`
	ServiceWorkerProbeTimeoutMS  int      `json:"service_worker_probe_timeout_ms,omitempty"`
	ServiceWorkerReadyTimeoutMS  int      `json:"service_worker_ready_timeout_ms,omitempty"`
	ServiceWorkerPollIntervalMS  int      `json:"service_worker_poll_interval_ms,omitempty"`
	TargetSessionPollIntervalMS  int      `json:"target_session_poll_interval_ms,omitempty"`
}

type ClientConfig struct {
	Routes               map[string]string `json:"routes,omitempty"`
	HydrateAliases       *bool             `json:"hydrate_aliases,omitempty"`
	MirrorUpstreamEvents *bool             `json:"mirror_upstream_events,omitempty"`
	CDPSendTimeoutMS     int               `json:"cdp_send_timeout_ms,omitempty"`
	EventWaitTimeoutMS   int               `json:"event_wait_timeout_ms,omitempty"`
}

type Options struct {
	Launch            LaunchConfig       `json:"launch,omitempty"`
	Upstream          UpstreamConfig     `json:"upstream,omitempty"`
	Extension         ExtensionConfig    `json:"extension,omitempty"`
	Client            ClientConfig       `json:"client,omitempty"`
	Server            *ServerConfig      `json:"server,omitempty"`
	CustomCommands    []CustomCommand    `json:"custom_commands,omitempty"`
	CustomEvents      []CustomEvent      `json:"custom_events,omitempty"`
	CustomMiddlewares []CustomMiddleware `json:"custom_middlewares,omitempty"`
	serverConfigured  bool
}

func (o *Options) UnmarshalJSON(data []byte) error {
	type optionsAlias Options
	var decoded optionsAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*o = Options(decoded)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rawServer, hasServer := raw["server"]
	if !hasServer {
		return nil
	}
	o.serverConfigured = true
	if strings.TrimSpace(string(rawServer)) == "null" {
		o.Server = nil
		return nil
	}
	var server ServerConfig
	if err := json.Unmarshal(rawServer, &server); err != nil {
		return err
	}
	o.Server = &server
	return nil
}

type Handler func(data any)

type handlerEntry struct {
	handler Handler
	pointer uintptr
}

type CDPEvent struct {
	Method       string         `json:"method"`
	Params       map[string]any `json:"params,omitempty"`
	CDPSessionID string         `json:"cdpSessionId,omitempty"`
	SessionID    string         `json:"sessionId,omitempty"`
}

type ModDomain struct {
	client *ModCDPClient
}

type ModCDPClient struct {
	Accessibility        AccessibilityDomain
	Animation            AnimationDomain
	Audits               AuditsDomain
	Autofill             AutofillDomain
	BackgroundService    BackgroundServiceDomain
	BluetoothEmulation   BluetoothEmulationDomain
	Browser              BrowserDomain
	CSS                  CSSDomain
	CacheStorage         CacheStorageDomain
	Cast                 CastDomain
	Console              ConsoleDomain
	DOM                  DOMDomain
	DOMDebugger          DOMDebuggerDomain
	DOMSnapshot          DOMSnapshotDomain
	DOMStorage           DOMStorageDomain
	Debugger             DebuggerDomain
	DeviceAccess         DeviceAccessDomain
	DeviceOrientation    DeviceOrientationDomain
	Emulation            EmulationDomain
	EventBreakpoints     EventBreakpointsDomain
	Extensions           ExtensionsDomain
	FedCm                FedCmDomain
	Fetch                FetchDomain
	FileSystem           FileSystemDomain
	HeadlessExperimental HeadlessExperimentalDomain
	HeapProfiler         HeapProfilerDomain
	IO                   IODomain
	IndexedDB            IndexedDBDomain
	Input                InputDomain
	Inspector            InspectorDomain
	LayerTree            LayerTreeDomain
	Log                  LogDomain
	Media                MediaDomain
	Memory               MemoryDomain
	Network              NetworkDomain
	Overlay              OverlayDomain
	PWA                  PWADomain
	Page                 PageDomain
	Performance          PerformanceDomain
	PerformanceTimeline  PerformanceTimelineDomain
	Preload              PreloadDomain
	Profiler             ProfilerDomain
	Runtime              RuntimeDomain
	Schema               SchemaDomain
	Security             SecurityDomain
	ServiceWorker        ServiceWorkerDomain
	SmartCardEmulation   SmartCardEmulationDomain
	Storage              StorageDomain
	SystemInfo           SystemInfoDomain
	Target               TargetDomain
	Tethering            TetheringDomain
	Tracing              TracingDomain
	WebAudio             WebAudioDomain
	WebAuthn             WebAuthnDomain
	Mod                  ModDomain

	Launch                  LaunchConfig
	Upstream                UpstreamConfig
	Extension               ExtensionConfig
	Client                  ClientConfig
	Server                  *ServerConfig
	CustomCommands          []CustomCommand
	CustomEvents            []CustomEvent
	CustomMiddlewares       []CustomMiddleware
	UpstreamEndpointKind    UpstreamEndpointKind
	CDPURL                  string
	transport               upstreamTransportClient
	mu                      sync.Mutex
	nextID                  int64
	pending                 map[int64]chan map[string]any
	handlers                map[string][]handlerEntry
	cdpHandlers             map[string][]func(CDPEvent)
	commandParamsSchemas    map[string]map[string]any
	commandResultSchemas    map[string]map[string]any
	commandResultUnwrapKeys map[string]string
	event_schemas           map[string]map[string]any
	schemaMu                sync.RWMutex
	handlersMu              sync.Mutex
	autoSessions            *AutoSessionRouter
	ExtensionID             string
	ExtTargetID             string
	ExtSessionID            string
	ExtExecutionContextID   int
	Latency                 map[string]any
	ConnectTiming           map[string]any
	LastCommandTiming       map[string]any
	LastRawTiming           map[string]any
	launchedBrowser         *LaunchedBrowser
	extensionInjectors      []extensionInjector
}

type extensionInjector interface {
	Update(ExtensionInjectorConfig) *ExtensionInjector
	GetLauncherConfig() LaunchOptions
	GetTransportConfig() map[string]any
	Prepare() error
	Inject() (*ExtensionInjectionResult, error)
	Close() error
}

type browserLauncherClient interface {
	Update(LaunchOptions) *BrowserLauncher
	GetInjectorConfig() ExtensionInjectorConfig
	GetTransportConfig() map[string]any
	Launch(LaunchOptions) (*LaunchedBrowser, error)
}

type upstreamTransportClient interface {
	Update(map[string]any)
	Connect() error
	Close() error
	Send(map[string]any) error
	GetLauncherConfig() LaunchOptions
	GetInjectorConfig() ExtensionInjectorConfig
	GetServerConfig() map[string]any
	OnRecv(func(map[string]any)) func()
	OnClose(func(error)) func()
	WaitForPeer() error
}

func New(opts Options) *ModCDPClient {
	if opts.Upstream.Mode == "" {
		opts.Upstream.Mode = "ws"
	}
	upstreamEndpointKind := UpstreamEndpointKindModCDPServer
	if opts.Upstream.Mode == "ws" || opts.Upstream.Mode == "pipe" {
		upstreamEndpointKind = UpstreamEndpointKindRawCDP
	}
	if opts.Launch.Mode == "" {
		if upstreamEndpointKind == UpstreamEndpointKindModCDPServer {
			opts.Launch.Mode = "none"
		} else if opts.Upstream.CDPURL != "" {
			opts.Launch.Mode = "remote"
		} else {
			opts.Launch.Mode = "local"
		}
	}
	if opts.Extension.Mode == "" {
		if upstreamEndpointKind == UpstreamEndpointKindRawCDP || opts.Launch.Mode != "none" {
			opts.Extension.Mode = "auto"
		} else {
			opts.Extension.Mode = "none"
		}
	}
	if opts.Launch.ExecutablePath != "" {
		opts.Launch.Options.ExecutablePath = opts.Launch.ExecutablePath
	}
	if opts.Launch.UserDataDir != "" {
		opts.Launch.Options.UserDataDir = opts.Launch.UserDataDir
	}
	if opts.Client.Routes == nil {
		opts.Client.Routes = translate.DefaultClientRoutes()
	} else {
		merged := translate.DefaultClientRoutes()
		for k, v := range opts.Client.Routes {
			merged[k] = v
		}
		opts.Client.Routes = merged
	}
	if opts.Client.HydrateAliases == nil {
		value := true
		opts.Client.HydrateAliases = &value
	}
	if opts.Server != nil && opts.Server.disabled {
		opts.Server = nil
		opts.serverConfigured = true
	}
	if opts.Server == nil && !opts.serverConfigured {
		opts.Server = &ServerConfig{}
	}
	if upstreamEndpointKind == UpstreamEndpointKindModCDPServer && opts.Server.Routes == nil {
		opts.Server.Routes = map[string]string{"*.*": "chrome_debugger"}
	}
	if opts.Extension.ServiceWorkerURLSuffixes == nil {
		opts.Extension.ServiceWorkerURLSuffixes = append([]string{}, DefaultModCDPServiceWorkerURLSuffixes...)
	}
	if opts.Client.CDPSendTimeoutMS == 0 {
		opts.Client.CDPSendTimeoutMS = DefaultCDPSendTimeoutMS
	}
	if opts.Client.EventWaitTimeoutMS == 0 {
		opts.Client.EventWaitTimeoutMS = DefaultEventWaitTimeoutMS
	}
	if opts.Extension.ExecutionContextTimeoutMS == 0 {
		opts.Extension.ExecutionContextTimeoutMS = DefaultExecutionContextTimeoutMS
	}
	if opts.Extension.ServiceWorkerProbeTimeoutMS == 0 {
		opts.Extension.ServiceWorkerProbeTimeoutMS = DefaultServiceWorkerProbeTimeoutMS
	}
	if opts.Extension.ServiceWorkerReadyTimeoutMS == 0 {
		opts.Extension.ServiceWorkerReadyTimeoutMS = DefaultServiceWorkerReadyTimeoutMS
	}
	if opts.Extension.ServiceWorkerPollIntervalMS == 0 {
		opts.Extension.ServiceWorkerPollIntervalMS = DefaultServiceWorkerPollIntervalMS
	}
	if opts.Extension.TargetSessionPollIntervalMS == 0 {
		opts.Extension.TargetSessionPollIntervalMS = DefaultTargetSessionPollIntervalMS
	}
	if opts.Upstream.WSConnectErrorSettleTimeoutMS == 0 {
		opts.Upstream.WSConnectErrorSettleTimeoutMS = DefaultWSConnectErrorSettleTimeoutMS
	}
	if opts.Upstream.UpstreamReverseWSBind == "" {
		opts.Upstream.UpstreamReverseWSBind = DefaultUpstreamReverseWSBind
	}
	if opts.Upstream.UpstreamNATSWaitTimeoutMS == 0 {
		opts.Upstream.UpstreamNATSWaitTimeoutMS = DefaultUpstreamNATSWaitTimeoutMS
	}
	if opts.Upstream.UpstreamReverseWSWaitTimeoutMS == 0 {
		opts.Upstream.UpstreamReverseWSWaitTimeoutMS = DefaultUpstreamReverseWSWaitTimeoutMS
	}
	if opts.Upstream.UpstreamNativeMessagingWaitTimeoutMS == 0 {
		opts.Upstream.UpstreamNativeMessagingWaitTimeoutMS = DefaultUpstreamNativeMessagingWaitTimeoutMS
	}
	client := &ModCDPClient{
		Launch:                  opts.Launch,
		Upstream:                opts.Upstream,
		Extension:               opts.Extension,
		Client:                  opts.Client,
		Server:                  opts.Server,
		CustomCommands:          opts.CustomCommands,
		CustomEvents:            opts.CustomEvents,
		CustomMiddlewares:       opts.CustomMiddlewares,
		UpstreamEndpointKind:    upstreamEndpointKind,
		pending:                 map[int64]chan map[string]any{},
		handlers:                map[string][]handlerEntry{},
		cdpHandlers:             map[string][]func(CDPEvent){},
		commandParamsSchemas:    map[string]map[string]any{},
		commandResultSchemas:    map[string]map[string]any{},
		commandResultUnwrapKeys: map[string]string{},
		event_schemas:           map[string]map[string]any{},
	}
	client.Mod = ModDomain{client: client}
	client.autoSessions = NewAutoSessionRouter(
		func(method string, params map[string]any, sessionID string) (map[string]any, error) {
			return client.sendMessage(method, params, sessionID)
		},
		func() int { return client.Extension.ExecutionContextTimeoutMS },
	)
	if *client.Client.HydrateAliases {
		initCDPSurface(client)
	}
	client.hydrateCustomSurface()
	return client
}

func (c *ModCDPClient) Connect() error {
	connectStartedAt := time.Now().UnixMilli()
	transportStartedAt := time.Now().UnixMilli()
	if err := c.connectUpstreamTransport(); err != nil {
		return err
	}
	transportConnectedAt := time.Now().UnixMilli()
	if c.transport == nil {
		return fmt.Errorf("upstream transport did not connect")
	}
	c.transport.OnRecv(func(message map[string]any) { c.handleMessage(message) })
	c.transport.OnClose(func(err error) { c.rejectAll(err) })
	if transportpkg.EndpointKindForUpstream(c.Upstream.Mode) == UpstreamEndpointKindModCDPServer {
		if err := c.transport.WaitForPeer(); err != nil {
			c.Close()
			return err
		}
		if c.Server != nil {
			if _, err := c.sendMessage("Mod.configure", c.serverConfigureParams(nil, nil, nil), ""); err != nil {
				c.Close()
				return err
			}
		}
		go func() { _ = c.measurePingLatency() }()
		connectedAt := time.Now().UnixMilli()
		c.ConnectTiming = map[string]any{
			"started_at":             connectStartedAt,
			"upstream_mode":          c.Upstream.Mode,
			"upstream_endpoint_kind": transportpkg.EndpointKindForUpstream(c.Upstream.Mode),
			"transport_started_at":   transportStartedAt,
			"transport_connected_at": transportConnectedAt,
			"transport_duration_ms":  transportConnectedAt - transportStartedAt,
			"connected_at":           connectedAt,
			"duration_ms":            connectedAt - connectStartedAt,
		}
		return nil
	}
	if err := c.initializeRawCDPTransport(); err != nil {
		c.Close()
		return err
	}
	extensionStartedAt := time.Now().UnixMilli()
	ext, err := c.injectExtension(c.extensionInjectors)
	if err != nil {
		c.Close()
		return err
	}
	extensionCompletedAt := time.Now().UnixMilli()
	c.ExtensionID = ext.ExtensionID
	c.ExtTargetID = ext.TargetID
	c.ExtSessionID = ext.SessionID
	if _, err := c.sendMessage("Runtime.enable", map[string]any{}, c.ExtSessionID); err != nil {
		c.Close()
		return err
	}
	extExecutionContextID, err := c.autoSessions.WaitForExecutionContext(c.ExtSessionID, c.Extension.ExecutionContextTimeoutMS)
	if err != nil {
		c.Close()
		return err
	}
	c.ExtExecutionContextID = extExecutionContextID
	if _, err := c.sendMessage("Runtime.addBinding", map[string]any{"name": translate.CustomEventBindingName}, c.ExtSessionID); err != nil {
		c.Close()
		return err
	}
	mirrorUpstreamEvents := true
	if c.Client.MirrorUpstreamEvents != nil {
		mirrorUpstreamEvents = *c.Client.MirrorUpstreamEvents
	}
	if mirrorUpstreamEvents {
		if _, err := c.sendMessage("Runtime.addBinding", map[string]any{"name": translate.UpstreamEventBindingName}, c.ExtSessionID); err != nil {
			c.Close()
			return err
		}
	}

	if c.Server != nil {
		customCommands := make([]map[string]any, 0, len(c.CustomCommands))
		for _, command := range c.CustomCommands {
			if command.Expression == "" {
				continue
			}
			customCommands = append(customCommands, map[string]any{
				"name":          command.Name,
				"expression":    command.Expression,
				"params_schema": command.ParamsSchema,
				"result_schema": command.ResultSchema,
			})
		}
		customEvents := make([]map[string]any, 0, len(c.CustomEvents))
		for _, event := range c.CustomEvents {
			customEvents = append(customEvents, map[string]any{
				"name":         event.Name,
				"event_schema": event.EventSchema,
			})
		}
		customMiddlewares := make([]map[string]any, 0, len(c.CustomMiddlewares))
		for _, middleware := range c.CustomMiddlewares {
			item := map[string]any{
				"phase":      middleware.Phase,
				"expression": middleware.Expression,
			}
			if middleware.Name != "" {
				item["name"] = middleware.Name
			}
			customMiddlewares = append(customMiddlewares, item)
		}
		configureParams := c.serverConfigureParams(customCommands, customEvents, customMiddlewares)
		command, err := translate.WrapCommandIfNeeded("Mod.configure", configureParams, c.Client.Routes, c.ExtSessionID)
		if err != nil {
			c.Close()
			return fmt.Errorf("Mod.configure: %w", err)
		}
		if _, err := c.sendRaw(command); err != nil {
			c.Close()
			return fmt.Errorf("Mod.configure: %w", err)
		}
	}
	go func() { _ = c.measurePingLatency() }()
	connectedAt := time.Now().UnixMilli()
	c.ConnectTiming = map[string]any{
		"started_at":             connectStartedAt,
		"upstream_mode":          c.Upstream.Mode,
		"upstream_endpoint_kind": transportpkg.EndpointKindForUpstream(c.Upstream.Mode),
		"transport_started_at":   transportStartedAt,
		"transport_connected_at": transportConnectedAt,
		"transport_duration_ms":  transportConnectedAt - transportStartedAt,
		"injector_source":        ext.Source,
		"injector_started_at":    extensionStartedAt,
		"injector_completed_at":  extensionCompletedAt,
		"injector_duration_ms":   extensionCompletedAt - extensionStartedAt,
		"connected_at":           connectedAt,
		"duration_ms":            connectedAt - connectStartedAt,
	}
	return nil
}

func (c *ModCDPClient) connectUpstreamTransport() error {
	if c.transport != nil {
		return nil
	}
	if !isKnownLaunchMode(c.Launch.Mode) {
		return fmt.Errorf("unknown launch.mode=%s", c.Launch.Mode)
	}
	if !isKnownUpstreamMode(c.Upstream.Mode) {
		return fmt.Errorf("unknown upstream.mode=%s", c.Upstream.Mode)
	}
	if !isKnownExtensionMode(c.Extension.Mode) {
		return fmt.Errorf("unknown extension.mode=%s", c.Extension.Mode)
	}
	launcher := c.browserLauncher()
	transport := c.upstreamTransport()
	injectors := c.extensionInjectorsForConfig()
	c.extensionInjectors = injectors
	initialTransportConfig := c.upstreamTransportConfig()

	transport.Update(initialTransportConfig)
	launcher.Update(c.Launch.Options)
	for _, injector := range injectors {
		injector.Update(c.baseExtensionInjectorConfig(nil))
	}
	for _, injector := range injectors {
		injector.Update(launcher.GetInjectorConfig())
	}
	for _, injector := range injectors {
		injector.Update(transport.GetInjectorConfig())
	}
	for _, injector := range injectors {
		if err := injector.Prepare(); err != nil {
			return err
		}
	}
	for _, injector := range injectors {
		launcher.Update(injector.GetLauncherConfig())
	}
	for _, injector := range injectors {
		transport.Update(injector.GetTransportConfig())
	}
	launcher.Update(transport.GetLauncherConfig())
	transport.Update(launcher.GetTransportConfig())

	if transportpkg.EndpointKindForUpstream(c.Upstream.Mode) == UpstreamEndpointKindModCDPServer {
		if err := transport.Connect(); err != nil {
			return err
		}
	}
	if c.Launch.Mode != "none" {
		launched, err := launcher.Launch(LaunchOptions{})
		if err != nil {
			_ = transport.Close()
			return err
		}
		c.launchedBrowser = launched
		transport.Update(launcher.GetTransportConfig())
		for _, injector := range injectors {
			injector.Update(launcher.GetInjectorConfig())
		}
		for _, injector := range injectors {
			transport.Update(injector.GetTransportConfig())
		}
	}
	launchedCDPURL := ""
	if c.launchedBrowser != nil {
		launchedCDPURL = c.launchedBrowser.CDPURL
	}
	if transportpkg.EndpointKindForUpstream(c.Upstream.Mode) == UpstreamEndpointKindRawCDP {
		if err := transport.Connect(); err != nil {
			return err
		}
	}

	c.transport = transport
	transportURL := transportURL(transport)
	if transportpkg.EndpointKindForUpstream(c.Upstream.Mode) == UpstreamEndpointKindRawCDP {
		c.CDPURL = firstNonEmptyString(transportURL, launchedCDPURL)
	} else {
		c.CDPURL = launchedCDPURL
	}
	if wsTransport, ok := transport.(*WebSocketUpstreamTransport); ok && wsTransport.URL != "" {
		// For ws mode, cdp_url has been resolved to the concrete WebSocket CDP endpoint after connect().
		c.Upstream.CDPURL = wsTransport.URL
	}

	serverConfig := map[string]any{}
	if transportpkg.EndpointKindForUpstream(c.Upstream.Mode) == UpstreamEndpointKindModCDPServer && launchedCDPURL != "" {
		serverConfig["loopback_cdp_url"] = launchedCDPURL
	}
	for key, value := range transport.GetServerConfig() {
		serverConfig[key] = value
	}
	if c.Server != nil {
		if loopbackCDPURL, _ := serverConfig["loopback_cdp_url"].(string); loopbackCDPURL != "" {
			initialCDPURL, _ := initialTransportConfig["cdp_url"].(string)
			if c.Server.LoopbackCDPURL == "" ||
				c.Server.LoopbackCDPURL == initialCDPURL ||
				c.Server.LoopbackCDPURL == launchedCDPURL {
				c.Server.LoopbackCDPURL = loopbackCDPURL
			}
		}
	}
	return nil
}

func (c *ModCDPClient) upstreamTransportConfig() map[string]any {
	return map[string]any{
		"cdp_url":                                  c.Upstream.CDPURL,
		"upstream_nats_url":                        c.Upstream.UpstreamNATSURL,
		"upstream_nats_subject_prefix":             c.Upstream.UpstreamNATSSubjectPrefix,
		"upstream_nats_wait_timeout_ms":            c.Upstream.UpstreamNATSWaitTimeoutMS,
		"upstream_reversews_bind":                  c.Upstream.UpstreamReverseWSBind,
		"upstream_reversews_wait_timeout_ms":       c.Upstream.UpstreamReverseWSWaitTimeoutMS,
		"upstream_nativemessaging_manifest":        c.Upstream.UpstreamNativeMessagingManifest,
		"upstream_nativemessaging_host_name":       c.Upstream.UpstreamNativeMessagingHostName,
		"upstream_nativemessaging_wait_timeout_ms": c.Upstream.UpstreamNativeMessagingWaitTimeoutMS,
		"extension_id":                             c.Extension.ExtensionID,
	}
}

func (c *ModCDPClient) initializeRawCDPTransport() error {
	if _, err := c.sendMessage("Target.setAutoAttach", map[string]any{
		"autoAttach":             true,
		"waitForDebuggerOnStart": false,
		"flatten":                true,
	}, ""); err != nil {
		return err
	}
	if _, err := c.sendMessage("Target.setDiscoverTargets", map[string]any{"discover": true}, ""); err != nil {
		return err
	}
	return nil
}

func transportURL(transport upstreamTransportClient) string {
	switch typed := transport.(type) {
	case *WebSocketUpstreamTransport:
		return typed.URL
	case *PipeUpstreamTransport:
		return typed.URL
	default:
		return ""
	}
}

func (c *ModCDPClient) serverConfigureParams(customCommands []map[string]any, customEvents []map[string]any, customMiddlewares []map[string]any) map[string]any {
	if customCommands == nil {
		customCommands = []map[string]any{}
	}
	if customEvents == nil {
		customEvents = []map[string]any{}
	}
	if customMiddlewares == nil {
		customMiddlewares = []map[string]any{}
	}
	server := map[string]any{
		"server_cdp_send_timeout_ms":                   c.Client.CDPSendTimeoutMS,
		"server_loopback_execution_context_timeout_ms": c.Extension.ExecutionContextTimeoutMS,
		"server_ws_connect_error_settle_timeout_ms":    c.Upstream.WSConnectErrorSettleTimeoutMS,
	}
	if c.Server != nil {
		server["server_loopback_cdp_url"] = c.Server.LoopbackCDPURL
		server["server_routes"] = c.Server.Routes
		if c.Server.BrowserToken != "" {
			server["server_browser_token"] = c.Server.BrowserToken
		}
		if c.Server.CDPSendTimeoutMS != 0 {
			server["server_cdp_send_timeout_ms"] = c.Server.CDPSendTimeoutMS
		}
		if c.Server.LoopbackExecutionContextTimeoutMS != 0 {
			server["server_loopback_execution_context_timeout_ms"] = c.Server.LoopbackExecutionContextTimeoutMS
		}
		if c.Server.WSConnectErrorSettleTimeoutMS != 0 {
			server["server_ws_connect_error_settle_timeout_ms"] = c.Server.WSConnectErrorSettleTimeoutMS
		}
		for key, value := range c.Server.Options {
			server[key] = value
		}
	}
	upstream := map[string]any{"upstream_mode": c.Upstream.Mode}
	if c.Upstream.UpstreamNATSURL != "" {
		upstream["upstream_nats_url"] = c.Upstream.UpstreamNATSURL
	}
	if c.Upstream.UpstreamNATSSubjectPrefix != "" {
		upstream["upstream_nats_subject_prefix"] = c.Upstream.UpstreamNATSSubjectPrefix
	}
	return map[string]any{
		"upstream": upstream,
		"client": map[string]any{
			"client_routes": c.Client.Routes,
		},
		"server":             server,
		"custom_commands":    customCommands,
		"custom_events":      customEvents,
		"custom_middlewares": customMiddlewares,
	}
}

func normalizeModCDPName(name string) (string, error) {
	normalized := strings.TrimSpace(name)
	if normalized == "" {
		return "", fmt.Errorf("name must be a non-empty string")
	}
	if strings.Count(normalized, ".") != 1 {
		return "", fmt.Errorf("name must be in Domain.method form")
	}
	parts := strings.Split(normalized, ".")
	if parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("name must be in Domain.method form")
	}
	return normalized, nil
}

func cloneSchema(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}
	normalized, _ := abxjsonschema.Normalize(schema).(map[string]any)
	if normalized == nil {
		return nil
	}
	return normalized
}

func resultUnwrapKeyFromSchema(schema map[string]any) string {
	properties, _ := schema["properties"].(map[string]any)
	if len(properties) != 1 {
		return ""
	}
	for key := range properties {
		return key
	}
	return ""
}

func (c *ModCDPClient) setCommandResultSchema(name string, schema map[string]any) {
	c.commandResultSchemas[name] = schema
	if unwrapKey := resultUnwrapKeyFromSchema(schema); unwrapKey != "" {
		c.commandResultUnwrapKeys[name] = unwrapKey
	} else {
		delete(c.commandResultUnwrapKeys, name)
	}
}

func (c *ModCDPClient) hydrateCustomSurface() {
	c.schemaMu.Lock()
	defer c.schemaMu.Unlock()
	for _, command := range c.CustomCommands {
		if command.Name == "" {
			continue
		}
		name, err := normalizeModCDPName(command.Name)
		if err != nil {
			continue
		}
		if schema := cloneSchema(command.ParamsSchema); schema != nil {
			c.commandParamsSchemas[name] = schema
		}
		if schema := cloneSchema(command.ResultSchema); schema != nil {
			c.setCommandResultSchema(name, schema)
		}
	}
	for _, event := range c.CustomEvents {
		if event.Name == "" {
			continue
		}
		name, err := normalizeModCDPName(event.Name)
		if err != nil {
			continue
		}
		if schema := cloneSchema(event.EventSchema); schema != nil {
			c.event_schemas[name] = schema
		}
	}
}

func (c *ModCDPClient) registerCustomCommandParams(params map[string]any) (string, bool, error) {
	rawName, _ := params["name"].(string)
	name, err := normalizeModCDPName(rawName)
	if err != nil {
		return "", false, err
	}
	c.schemaMu.Lock()
	defer c.schemaMu.Unlock()
	if rawSchema, exists := params["params_schema"]; exists {
		schemaObject, ok := rawSchema.(map[string]any)
		if !ok {
			return "", false, fmt.Errorf("params_schema must be a JSON Schema object")
		}
		if schema := cloneSchema(schemaObject); schema != nil {
			c.commandParamsSchemas[name] = schema
		}
	}
	if rawSchema, exists := params["result_schema"]; exists {
		schemaObject, ok := rawSchema.(map[string]any)
		if !ok {
			return "", false, fmt.Errorf("result_schema must be a JSON Schema object")
		}
		if schema := cloneSchema(schemaObject); schema != nil {
			c.setCommandResultSchema(name, schema)
		}
	}
	expression, _ := params["expression"].(string)
	return name, expression != "", nil
}

func (c *ModCDPClient) registerCustomEventParams(params map[string]any) (string, error) {
	rawName, _ := params["name"].(string)
	name, err := normalizeModCDPName(rawName)
	if err != nil {
		return "", err
	}
	c.schemaMu.Lock()
	defer c.schemaMu.Unlock()
	if rawSchema, exists := params["event_schema"]; exists {
		schemaObject, ok := rawSchema.(map[string]any)
		if !ok {
			return "", fmt.Errorf("event_schema must be a JSON Schema object")
		}
		if schema := cloneSchema(schemaObject); schema != nil {
			c.event_schemas[name] = schema
		}
	}
	return name, nil
}

func (c *ModCDPClient) validateCommandParams(method string, params map[string]any) error {
	c.schemaMu.RLock()
	schema := c.commandParamsSchemas[method]
	c.schemaMu.RUnlock()
	if schema == nil {
		return nil
	}
	if err := abxjsonschema.Validate(schema, params); err != nil {
		return fmt.Errorf("%s params did not match params_schema: %w", method, err)
	}
	return nil
}

func (c *ModCDPClient) validateCommandResult(method string, result any) error {
	c.schemaMu.RLock()
	schema := c.commandResultSchemas[method]
	c.schemaMu.RUnlock()
	if schema == nil {
		return nil
	}
	if err := abxjsonschema.Validate(schema, result); err != nil {
		return fmt.Errorf("%s result did not match result_schema: %w", method, err)
	}
	return nil
}

func (c *ModCDPClient) validateAndUnwrapCommandResult(method string, result any) (any, error) {
	if err := c.validateCommandResult(method, result); err != nil {
		return nil, err
	}
	c.schemaMu.RLock()
	unwrapKey := c.commandResultUnwrapKeys[method]
	c.schemaMu.RUnlock()
	if unwrapKey == "" {
		return result, nil
	}
	resultObject, ok := result.(map[string]any)
	if !ok {
		return result, nil
	}
	return resultObject[unwrapKey], nil
}

func (c *ModCDPClient) validateEventData(event string, data any) (any, bool) {
	c.schemaMu.RLock()
	schema := c.event_schemas[event]
	c.schemaMu.RUnlock()
	if schema == nil {
		return data, true
	}
	if err := abxjsonschema.Validate(schema, data); err != nil {
		fmt.Fprintf(os.Stderr, "[ModCDPClient] %s event did not match event_schema: %v\n", event, err)
		return nil, false
	}
	return data, true
}

func (c *ModCDPClient) Send(method string, params map[string]any, sessionID ...string) (any, error) {
	targetSessionID := ""
	if len(sessionID) > 0 {
		targetSessionID = sessionID[0]
	}
	return c.sendCommand(method, params, targetSessionID, true)
}

func (d ModDomain) Evaluate(params map[string]any) (any, error) {
	return d.client.Send("Mod.evaluate", params)
}

func (d ModDomain) AddCustomCommand(params CustomCommand) (any, error) {
	commandParams := map[string]any{"name": params.Name}
	if params.Expression != "" {
		commandParams["expression"] = params.Expression
	}
	if params.ParamsSchema != nil {
		commandParams["params_schema"] = params.ParamsSchema
	}
	if params.ResultSchema != nil {
		commandParams["result_schema"] = params.ResultSchema
	}
	return d.client.Send("Mod.addCustomCommand", commandParams)
}

func (d ModDomain) AddCustomEvent(params CustomEvent) (any, error) {
	eventParams := map[string]any{"name": params.Name}
	if params.EventSchema != nil {
		eventParams["event_schema"] = params.EventSchema
	}
	return d.client.Send("Mod.addCustomEvent", eventParams)
}

func (d ModDomain) AddMiddleware(params CustomMiddleware) (any, error) {
	middlewareParams := map[string]any{
		"phase":      params.Phase,
		"expression": params.Expression,
	}
	if params.Name != "" {
		middlewareParams["name"] = params.Name
	}
	return d.client.Send("Mod.addMiddleware", middlewareParams)
}

func (d ModDomain) Configure(params map[string]any) (any, error) {
	return d.client.Send("Mod.configure", params)
}

func (d ModDomain) Ping(params map[string]any) (any, error) {
	return d.client.Send("Mod.ping", params)
}

func (c *ModCDPClient) sendCommand(method string, params map[string]any, targetSessionID string, validateSchema bool) (any, error) {
	startedAt := time.Now().UnixMilli()
	if params == nil {
		params = map[string]any{}
	}
	if method == "Mod.addCustomCommand" {
		name, hasExpression, err := c.registerCustomCommandParams(params)
		if err != nil {
			return nil, err
		}
		if !hasExpression {
			completedAt := time.Now().UnixMilli()
			c.LastCommandTiming = map[string]any{
				"method":       method,
				"target":       "client",
				"started_at":   startedAt,
				"completed_at": completedAt,
				"duration_ms":  completedAt - startedAt,
			}
			return map[string]any{"name": name, "registered": true}, nil
		}
	} else if method == "Mod.addCustomEvent" {
		name, err := c.registerCustomEventParams(params)
		if err != nil {
			return nil, err
		}
		if c.ExtSessionID == "" {
			completedAt := time.Now().UnixMilli()
			c.LastCommandTiming = map[string]any{
				"method":       method,
				"target":       "client",
				"started_at":   startedAt,
				"completed_at": completedAt,
				"duration_ms":  completedAt - startedAt,
			}
			return map[string]any{"name": name, "registered": true}, nil
		}
	}
	if validateSchema {
		if err := c.validateCommandParams(method, params); err != nil {
			return nil, err
		}
	}
	if transportpkg.EndpointKindForUpstream(c.Upstream.Mode) == UpstreamEndpointKindModCDPServer {
		rawResult, err := c.sendMessage(method, params, "")
		var result any = rawResult
		completedAt := time.Now().UnixMilli()
		c.LastCommandTiming = map[string]any{
			"method":       method,
			"target":       "modcdp_server",
			"started_at":   startedAt,
			"completed_at": completedAt,
			"duration_ms":  completedAt - startedAt,
		}
		if err != nil {
			return nil, err
		}
		if validateSchema {
			var err error
			result, err = c.validateAndUnwrapCommandResult(method, result)
			if err != nil {
				return nil, err
			}
		}
		return result, nil
	}
	command, err := translate.WrapCommandIfNeeded(method, params, c.Client.Routes, c.ExtSessionID, targetSessionID)
	if err != nil {
		return nil, err
	}
	result, err := c.sendRaw(command)
	completedAt := time.Now().UnixMilli()
	c.LastCommandTiming = map[string]any{
		"method":       method,
		"target":       command.Target,
		"started_at":   startedAt,
		"completed_at": completedAt,
		"duration_ms":  completedAt - startedAt,
	}
	if err != nil {
		return nil, err
	}
	if validateSchema {
		var err error
		result, err = c.validateAndUnwrapCommandResult(method, result)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (c *ModCDPClient) SendRaw(method string, params map[string]any, sessionID ...string) (map[string]any, error) {
	startedAt := time.Now().UnixMilli()
	if params == nil {
		params = map[string]any{}
	}
	targetSessionID := ""
	if len(sessionID) > 0 {
		targetSessionID = sessionID[0]
	}
	result, err := c.sendMessage(method, params, targetSessionID)
	completedAt := time.Now().UnixMilli()
	c.LastRawTiming = map[string]any{
		"method":       method,
		"started_at":   startedAt,
		"completed_at": completedAt,
		"duration_ms":  completedAt - startedAt,
	}
	return result, err
}

func (c *ModCDPClient) OnRaw(event string, handler Handler) *ModCDPClient {
	return c.On(event, handler)
}

func (c *ModCDPClient) OnCDP(event string, handler func(CDPEvent)) *ModCDPClient {
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()
	c.cdpHandlers[event] = append(c.cdpHandlers[event], handler)
	return c
}

func (c *ModCDPClient) On(event string, handler Handler) *ModCDPClient {
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()
	pointer := handlerPointer(handler)
	for _, existing := range c.handlers[event] {
		if existing.pointer == pointer {
			return c
		}
	}
	c.handlers[event] = append(c.handlers[event], handlerEntry{handler: handler, pointer: pointer})
	return c
}

func (c *ModCDPClient) Once(event string, handler Handler) *ModCDPClient {
	var wrapped Handler
	wrapped = func(data any) {
		c.Off(event, wrapped)
		handler(data)
	}
	return c.On(event, wrapped)
}

func (c *ModCDPClient) Off(event string, handler Handler) *ModCDPClient {
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()
	pointer := handlerPointer(handler)
	entries := c.handlers[event]
	filtered := entries[:0]
	for _, entry := range entries {
		if entry.pointer != pointer {
			filtered = append(filtered, entry)
		}
	}
	if len(filtered) == 0 {
		delete(c.handlers, event)
	} else {
		c.handlers[event] = filtered
	}
	return c
}

func handlerPointer(handler Handler) uintptr {
	if handler == nil {
		return 0
	}
	return reflect.ValueOf(handler).Pointer()
}

func (c *ModCDPClient) Close() {
	if c.transport != nil {
		_ = c.transport.Close()
		c.transport = nil
	}
	if c.launchedBrowser != nil {
		c.launchedBrowser.Close()
		c.launchedBrowser = nil
	}
	for _, injector := range c.extensionInjectors {
		_ = injector.Close()
	}
	c.extensionInjectors = nil
}

func (c *ModCDPClient) Transport() any {
	return c.transport
}

func (c *ModCDPClient) LaunchedBrowser() *LaunchedBrowser {
	return c.launchedBrowser
}

func (c *ModCDPClient) browserLauncher() browserLauncherClient {
	switch c.Launch.Mode {
	case "local":
		return NewLocalBrowserLauncher(c.Launch.Options)
	case "remote":
		return NewRemoteBrowserLauncher(c.Launch.Options, c.Upstream.CDPURL)
	case "bb":
		return NewBrowserbaseBrowserLauncher(c.Launch.Options)
	case "none":
		return NewNoopBrowserLauncher(c.Launch.Options)
	default:
		return nil
	}
}

func (c *ModCDPClient) upstreamTransport() upstreamTransportClient {
	switch c.Upstream.Mode {
	case "ws":
		return NewWebSocketUpstreamTransport(WebSocketUpstreamTransportOptions{})
	case "pipe":
		return NewPipeUpstreamTransport(PipeUpstreamTransportOptions{})
	case "reversews":
		return NewReverseWebSocketUpstreamTransport(ReverseWebSocketUpstreamTransportOptions{})
	case "nativemessaging":
		return NewNativeMessagingUpstreamTransport(NativeMessagingUpstreamTransportOptions{})
	case "nats":
		return NewNatsUpstreamTransport(NatsUpstreamTransportOptions{})
	default:
		return nil
	}
}

func (c *ModCDPClient) extensionInjectorsForConfig() []extensionInjector {
	if c.Extension.Mode == "none" {
		return nil
	}
	var injectors []extensionInjector
	if c.Extension.Mode == "auto" || c.Extension.Mode == "discover" {
		injector := NewDiscoveredExtensionInjector(ExtensionInjectorConfig{})
		injectors = append(injectors, &injector)
	}
	if c.Extension.Mode == "auto" || c.Extension.Mode == "inject" {
		if c.Launch.Mode == "bb" {
			injector := NewBBBrowserExtensionInjector(ExtensionInjectorConfig{})
			injectors = append(injectors, &injector)
		}
		if c.Launch.Mode == "local" {
			injector := NewLocalBrowserLaunchExtensionInjector(ExtensionInjectorConfig{})
			injectors = append(injectors, &injector)
		}
		injector := NewExtensionsLoadUnpackedInjector(ExtensionInjectorConfig{})
		injectors = append(injectors, &injector)
	}
	if c.Extension.Mode == "auto" || c.Extension.Mode == "borrow" {
		injector := NewBorrowedExtensionInjector(ExtensionInjectorConfig{})
		injectors = append(injectors, &injector)
	}
	return injectors
}

func isKnownLaunchMode(mode string) bool {
	return mode == "local" || mode == "remote" || mode == "bb" || mode == "none"
}

func isKnownUpstreamMode(mode string) bool {
	return mode == "ws" || mode == "pipe" || mode == "nativemessaging" || mode == "reversews" || mode == "nats"
}

func isKnownExtensionMode(mode string) bool {
	return mode == "auto" || mode == "discover" || mode == "inject" || mode == "borrow" || mode == "none"
}

func (c *ModCDPClient) baseExtensionInjectorConfig(send SendCDP) ExtensionInjectorConfig {
	trustMatchedServiceWorker := c.trustServiceWorkerTarget()
	var attachToTarget AttachToTarget
	if send != nil {
		attachToTarget = func(targetID string) string {
			return c.ensureSessionIDForTarget(targetID, time.Duration(c.Extension.ServiceWorkerProbeTimeoutMS)*time.Millisecond, true)
		}
	}
	return ExtensionInjectorConfig{
		Send:               send,
		SessionIDForTarget: func(targetID string) string { return c.autoSessions.SessionIDForTarget(targetID) },
		AttachToTarget:     attachToTarget,
		WaitForExecutionContext: func(sessionID string, timeoutMS int) int {
			contextID, _ := c.autoSessions.WaitForExecutionContext(sessionID, timeoutMS)
			return contextID
		},
		ExtensionPath:                c.Extension.Path,
		ExtensionID:                  c.Extension.ExtensionID,
		WakePath:                     firstNonEmptyString(c.Extension.WakePath, DefaultModCDPWakePath),
		WakeURL:                      c.Extension.WakeURL,
		ServiceWorkerURLIncludes:     c.Extension.ServiceWorkerURLIncludes,
		ServiceWorkerURLSuffixes:     c.Extension.ServiceWorkerURLSuffixes,
		TrustServiceWorkerTarget:     trustMatchedServiceWorker,
		RequireServiceWorkerTarget:   c.Extension.RequireServiceWorkerTarget || c.Extension.Mode == "discover",
		ServiceWorkerReadyExpression: c.Extension.ServiceWorkerReadyExpression,
		CDPSendTimeoutMS:             c.Client.CDPSendTimeoutMS,
		ExecutionContextTimeoutMS:    c.Extension.ExecutionContextTimeoutMS,
		ServiceWorkerProbeTimeoutMS:  c.Extension.ServiceWorkerProbeTimeoutMS,
		ServiceWorkerReadyTimeoutMS:  c.Extension.ServiceWorkerReadyTimeoutMS,
		ServiceWorkerPollIntervalMS:  c.Extension.ServiceWorkerPollIntervalMS,
		TargetSessionPollIntervalMS:  c.Extension.TargetSessionPollIntervalMS,
	}
}

func (c *ModCDPClient) injectExtension(injectors []extensionInjector) (*ExtensionInjectionResult, error) {
	if len(injectors) == 0 {
		return nil, fmt.Errorf("extension.mode='none' cannot be used with a raw_cdp upstream")
	}
	send := func(method string, params map[string]any, sessionID string) (map[string]any, error) {
		return c.sendMessageTimeout(method, params, sessionID, time.Duration(c.Client.CDPSendTimeoutMS)*time.Millisecond)
	}
	var errors []string
	for _, injector := range injectors {
		injector.Update(c.baseExtensionInjectorConfig(send))
		if err := injector.Prepare(); err != nil {
			errors = append(errors, fmt.Sprintf("%T: %v", injector, err))
			continue
		}
		result, err := injector.Inject()
		if err != nil {
			errors = append(errors, fmt.Sprintf("%T: %v", injector, err))
			continue
		}
		if result != nil {
			return result, nil
		}
	}
	return nil, fmt.Errorf("cannot install, discover, or borrow the ModCDP extension in the running browser.%s", formatInjectorErrors(errors))
}

func formatInjectorErrors(errors []string) string {
	if len(errors) == 0 {
		return ""
	}
	return "\n\n" + strings.Join(errors, "\n")
}

func cloneMap(value map[string]any) map[string]any {
	cloned := map[string]any{}
	for key, item := range value {
		cloned[key] = item
	}
	return cloned
}

func (c *ModCDPClient) sendRaw(command translate.RawCommand) (any, error) {
	if command.Target == "direct_cdp" {
		step := command.Steps[0]
		return c.sendMessage(step.Method, step.Params, step.SessionID)
	}
	if command.Target != "service_worker" {
		return nil, fmt.Errorf("unsupported command target %q", command.Target)
	}

	var result map[string]any
	unwrap := ""
	for _, step := range command.Steps {
		params := step.Params
		if params == nil {
			params = map[string]any{}
		}
		if step.Method == "Runtime.callFunctionOn" {
			if _, exists := params["executionContextId"]; !exists {
				if c.ExtExecutionContextID == 0 {
					contextID, err := c.autoSessions.WaitForExecutionContext(c.ExtSessionID, c.Extension.ExecutionContextTimeoutMS)
					if err != nil {
						return nil, err
					}
					c.ExtExecutionContextID = contextID
				}
				params = cloneMap(params)
				params["executionContextId"] = c.ExtExecutionContextID
			}
		}
		r, err := c.sendMessage(step.Method, params, c.ExtSessionID)
		if err != nil {
			return nil, err
		}
		result = r
		unwrap = step.Unwrap
	}
	return translate.UnwrapResponseIfNeeded(result, unwrap)
}

func (c *ModCDPClient) measurePingLatency() error {
	sent_at := time.Now().UnixMilli()
	ch := make(chan any, 1)
	c.Once("Mod.pong", func(data any) {
		select {
		case ch <- data:
		default:
		}
	})
	if _, err := c.Send("Mod.ping", map[string]any{"sent_at": sent_at}); err != nil {
		return err
	}
	select {
	case payload := <-ch:
		returned_at := time.Now().UnixMilli()
		latency := map[string]any{
			"sent_at":           sent_at,
			"received_at":       nil,
			"returned_at":       returned_at,
			"round_trip_ms":     returned_at - sent_at,
			"service_worker_ms": nil,
			"return_path_ms":    nil,
		}
		if data, ok := payload.(map[string]any); ok {
			if received_at, ok := numberAsInt64(data["received_at"]); ok {
				latency["received_at"] = received_at
				latency["service_worker_ms"] = received_at - sent_at
				latency["return_path_ms"] = returned_at - received_at
			}
		}
		c.Latency = latency
		return nil
	case <-time.After(time.Duration(c.Client.EventWaitTimeoutMS) * time.Millisecond):
		return fmt.Errorf("Mod.pong timed out")
	}
}

func numberAsInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		return int64(v), true
	default:
		return 0, false
	}
}

func (c *ModCDPClient) sendMessage(method string, params map[string]any, sessionID string) (map[string]any, error) {
	return c.sendMessageTimeout(method, params, sessionID, time.Duration(c.Client.CDPSendTimeoutMS)*time.Millisecond)
}

func (c *ModCDPClient) sendMessageTimeout(method string, params map[string]any, sessionID string, timeout time.Duration) (map[string]any, error) {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	ch := make(chan map[string]any, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	msg := map[string]any{"id": id, "method": method, "params": params}
	if sessionID != "" {
		msg["sessionId"] = sessionID
	}
	var err error
	if c.transport != nil {
		err = c.transport.Send(msg)
	} else {
		err = fmt.Errorf("ModCDP upstream is not connected")
	}
	if err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, err
	}
	if timeout <= 0 {
		resp := <-ch
		if errObj, ok := resp["error"].(map[string]any); ok {
			return nil, fmt.Errorf("%s failed: %v", method, errObj["message"])
		}
		if r, ok := resp["result"].(map[string]any); ok {
			return r, nil
		}
		return map[string]any{}, nil
	}
	select {
	case <-time.After(timeout):
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("%s timed out after %s", method, timeout)
	case resp := <-ch:
		if errObj, ok := resp["error"].(map[string]any); ok {
			return nil, fmt.Errorf("%s failed: %v", method, errObj["message"])
		}
		if r, ok := resp["result"].(map[string]any); ok {
			return r, nil
		}
		return map[string]any{}, nil
	}
}

func (c *ModCDPClient) rejectAll(err error) {
	c.mu.Lock()
	pending := c.pending
	c.pending = map[int64]chan map[string]any{}
	c.mu.Unlock()
	for _, ch := range pending {
		ch <- map[string]any{"error": map[string]any{"message": fmt.Sprintf("connection closed: %v", err)}}
	}
}

func (c *ModCDPClient) handleMessage(msg map[string]any) {
	if idF, ok := msg["id"].(float64); ok {
		id := int64(idF)
		c.mu.Lock()
		ch, ok := c.pending[id]
		delete(c.pending, id)
		c.mu.Unlock()
		if ok {
			ch <- msg
		}
		return
	}
	if id, ok := msg["id"].(int); ok {
		c.mu.Lock()
		ch, found := c.pending[int64(id)]
		delete(c.pending, int64(id))
		c.mu.Unlock()
		if found {
			ch <- msg
		}
		return
	}
	c.handleEventMessage(msg)
}

func (c *ModCDPClient) handleEventMessage(msg map[string]any) {
	method, _ := msg["method"].(string)
	sessionID, _ := msg["sessionId"].(string)
	params, _ := msg["params"].(map[string]any)
	c.autoSessions.RecordProtocolEvent(method, params, sessionID)
	if c.ExtSessionID != "" && sessionID == c.ExtSessionID {
		bindingName, _ := params["name"].(string)
		if event, data, ok := translate.UnwrapEventIfNeeded(method, params, sessionID, c.ExtSessionID); ok {
			validatedData, valid := c.validateEventData(event, data)
			if !valid {
				return
			}
			c.handlersMu.Lock()
			hs := append([]handlerEntry(nil), c.handlers[event]...)
			cdpHandlers := append([]func(CDPEvent){}, c.cdpHandlers["*"]...)
			cdpHandlers = append(cdpHandlers, c.cdpHandlers[event]...)
			c.handlersMu.Unlock()
			for _, h := range hs {
				go h.handler(validatedData)
			}
			if bindingName == translate.UpstreamEventBindingName {
				dataMap, _ := validatedData.(map[string]any)
				cdpEvent := CDPEvent{Method: event, Params: dataMap, CDPSessionID: sessionID, SessionID: sessionID}
				for _, h := range cdpHandlers {
					go h(cdpEvent)
				}
			}
		}
		return
	}
	if method != "" {
		validatedParams, valid := c.validateEventData(method, params)
		if !valid {
			return
		}
		validatedParamsMap, _ := validatedParams.(map[string]any)
		if validatedParamsMap == nil {
			validatedParamsMap = map[string]any{}
		}
		c.handlersMu.Lock()
		hs := append([]handlerEntry(nil), c.handlers[method]...)
		cdpHandlers := append([]func(CDPEvent){}, c.cdpHandlers["*"]...)
		cdpHandlers = append(cdpHandlers, c.cdpHandlers[method]...)
		c.handlersMu.Unlock()
		for _, h := range hs {
			go h.handler(validatedParams)
		}
		if len(cdpHandlers) > 0 {
			event := CDPEvent{Method: method, Params: validatedParamsMap, CDPSessionID: sessionID, SessionID: sessionID}
			for _, h := range cdpHandlers {
				go h(event)
			}
		}
	}
}

func (c *ModCDPClient) trustServiceWorkerTarget() bool {
	if c.Extension.TrustServiceWorkerTarget || len(c.Extension.ServiceWorkerURLIncludes) > 0 {
		return true
	}
	for _, suffix := range c.Extension.ServiceWorkerURLSuffixes {
		parts := 0
		for _, part := range strings.Split(suffix, "/") {
			if part != "" {
				parts++
			}
		}
		if parts > 1 {
			return true
		}
	}
	return false
}

func (c *ModCDPClient) sessionIDForTarget(targetID string, timeout time.Duration) string {
	if timeout <= 0 {
		return c.autoSessions.SessionIDForTarget(targetID)
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline.Add(time.Millisecond)) {
		sessionID := c.autoSessions.SessionIDForTarget(targetID)
		if sessionID != "" {
			return sessionID
		}
		time.Sleep(time.Duration(c.Extension.TargetSessionPollIntervalMS) * time.Millisecond)
	}
	return ""
}

func (c *ModCDPClient) ensureSessionIDForTarget(targetID string, timeout time.Duration, allowAttach bool) string {
	sessionID := c.autoSessions.SessionIDForTarget(targetID)
	if sessionID != "" {
		return sessionID
	}
	if allowAttach {
		attachedSessionID := c.autoSessions.AttachToTarget(targetID)
		if attachedSessionID != "" {
			return attachedSessionID
		}
	}
	return c.sessionIDForTarget(targetID, timeout)
}
