# ModCDP ChromeOS Extension Fork - Change Log

This document details the modifications made to ModCDP to support ChromeOS Crostini extension connectivity via secure WebSockets (WSS).

## Overview

The primary goal of these changes was to enable ModCDP to work with ChromeOS extensions, which require secure WebSocket (WSS) connections in Chrome 144+. The changes involved modifications to both the proxy server and the extension source code.

## Commits Summary

| Commit | Date | Description |
|--------|------|-------------|
| 47ed6e1 | 2026-07-20 | Remove NATS and NativeMessaging from extension - use ReverseWS only |
| 2e15aab | 2026-07-20 | Add WSS support and 0.0.0.0 binding for ChromeOS Crostini |
| 6c75ec6 | 2026-07-20 | Add extension error reporting to proxy debug endpoint |
| a593e8b | 2026-07-20 | Correct debug endpoint placement in proxy.ts |
| 39d65fe | 2026-07-20 | Add status reporting for extension connection diagnosis |
| 7c1ad67 | 2026-07-20 | Add error logging to ReverseWSDownstreamTransport for connection diagnosis |

---

## Proxy Changes

### `js/src/proxy/proxy.ts`

#### 1. Changed DEFAULT_HOST binding (line 46)
```typescript
// Before:
const DEFAULT_HOST = "127.0.0.1";

// After:
const DEFAULT_HOST = "0.0.0.0"; // Changed to 0.0.0.0 for ChromeOS localhost forwarding
```
**Reason:** ChromeOS Crostini localhost forwarding requires the proxy to listen on all interfaces (0.0.0.0) rather than just 127.0.0.1.

#### 2. Added shared upstream transport preconnect hack (lines 83-97)
```typescript
// PRECONNECT HACK: For reversews mode, create shared upstream listener so extension can connect immediately
// This prevents the race condition where downstream clients can't connect until the extension does
let shared_upstream_transport: ReverseWSUpstreamTransport | null = null;

if (upstream.upstream_mode === "reversews") {
  shared_upstream_transport = new ReverseWSUpstreamTransport(upstream);
  shared_upstream_transport.connect().then(() => {
    console.log("Upstream reversews listener ready on " + (upstream.upstream_reversews_bind || "127.0.0.1:29292"));
  }).catch((e) => {
    console.error("Failed to start reversews listener:", e?.message || e);
  });
}
```
**Reason:** This prevents a race condition where downstream CDP clients cannot connect until the extension establishes its upstream connection first. By pre-starting the listener, the extension can connect immediately and the proxy can accept downstream connections that wait for the extension peer.

#### 3. Added `/debug-report` HTTP endpoint (lines 120-139)
```typescript
// Debug endpoint: extension can report connection status here
if (req.url === "/debug-report" && req.method === "POST") {
  let body = "";
  req.on("data", (chunk) => (body += chunk));
  req.on("end", () => {
    try {
      const data = JSON.parse(body);
      console.log("[Extension Report]", JSON.stringify(data));
      fs.writeFileSync(
        path.join(process.cwd(), "js/proxy/extension-status.json"),
        JSON.stringify({ ...data, timestamp: Date.now() }, null, 2)
      );
    } catch (e) {
      console.error("[Extension Report] Parse error:", e);
    }
    res.writeHead(200);
    res.end("OK");
  });
  return;
}
```
**Reason:** Provides visibility into extension connection status and errors for debugging purposes. The extension reports connection attempts, errors, and status via HTTP POST to this endpoint.

#### 4. Modified `connectDownstream` function signature (line 188)
Added optional `shared_upstream_transport` parameter to support shared transport functionality.

#### 5. Added shared transport handling in `connectDownstream` (lines 193-248)
```typescript
// For shared transport, we still need to connect but the transport is already listening
// We need to wait for the extension peer to connect
if (shared_upstream_transport) {
  try {
    await shared_upstream_transport.waitForPeer();
    connected = true;
    for (const raw of queued_raw_messages.splice(0)) void handleDownstreamMessage(socket, cdp, raw);
  } catch (error) {
    // Peer timeout - wait and retry a bit
    console.error(`[ModCDP proxy] waiting for extension peer: ${errorMessage(error)}`);
    // Keep connection open and wait for peer, don't close socket immediately
    shared_upstream_transport.on("*", (event_name, payload, session_id) => {
      if (socket.readyState !== socket.OPEN) return;
      socket.send(JSON.stringify({
        method: String(event_name),
        params: payload ?? {},
        ...(session_id ? { sessionId: session_id } : {}),
      }));
    });
    // ... background peer waiting logic
  }
  return;
}
```
**Reason:** When using shared transport, downstream connections wait for the extension peer rather than immediately failing.

#### 6. Added console logging for CDP commands (line 276)
```typescript
console.log(`[Proxy] Received CDP command: ${message.method}`);
```
Added for debugging and visibility into proxy operations.

---

### `js/src/transport/ReverseWSUpstreamTransport.ts`

#### 1. Added HTTPS/WSS support with certificate loading (lines 10-11, 13-134)
```typescript
import fs from "node:fs";
import https from "node:https";

const DEFAULT_UPSTREAM_REVERSEWS_BIND = "0.0.0.0:29292"; // Listen on all interfaces for ChromeOS
const DEFAULT_UPSTREAM_REVERSEWS_CERT_DIR = "/home/kiersten/code/ModCDP/js/proxy"; // Cert directory for WSS
```

**Modified `connect()` method to support WSS:**
```typescript
async connect() {
  const { WebSocketServer } = await import("ws");
  const { host, port } = parseHostPort(this.config.upstream_reversews_bind, "127.0.0.1", 29292);
  
  // Support WSS if cert directory is provided or default cert exists
  const certDir = this.config.upstream_reversews_cert_dir ?? DEFAULT_UPSTREAM_REVERSEWS_CERT_DIR;
  if (certDir && fs.existsSync(`${certDir}/cert.pem`)) {
    try {
      const cert = fs.readFileSync(`${certDir}/cert.pem`);
      const key = fs.readFileSync(`${certDir}/key.pem`);
      this.endpoint_url = `wss://${host}:${port}`;
      const httpsServer = https.createServer({ cert, key });
      await new Promise<void>((resolve, reject) => {
        httpsServer.listen(port, host, (err) => err ? reject(err) : resolve());
      });
      const reversews_listener = new WebSocketServer({ server: httpsServer });
      this.reversews_listener = reversews_listener;
    } catch (e) {
      console.warn("Failed to create HTTPS server, falling back to WS", e);
    }
  }
  // ... fallback to WS if cert not found
}
```
**Reason:** ChromeOS extensions require WSS (secure WebSocket) connections. The proxy now supports TLS by loading mkcert-generated certificates.

#### 2. Added console logging for hello reception (lines 145-147, 199)
```typescript
console.log("[ModCDP proxy] waitForPeer: existing peer available");
// ...
console.log("[ModCDP proxy] received reverse hello:", JSON.stringify(hello));
```
**Reason:** Debugging visibility to understand when the extension connects and sends its hello message.

#### 3. Added timeout warning log (line 189)
```typescript
console.warn("[ModCDP proxy] reverse hello timeout");
```
**Reason:** Helps diagnose connection issues when the extension doesn't send a hello within the timeout period.

---

### `js/src/client/ModCDPClient.ts`

#### 1. Added `_existing_upstream` parameter (lines 79-80, 189-197)
```typescript
/** @internal Use existing upstream transport instead of creating one */
  _existing_upstream?: UpstreamTransport;
```

```typescript
// Use existing upstream transport if provided (for sharing in proxy mode)
if ((config as any)._existing_upstream) {
  this.upstream = (config as any)._existing_upstream;
} else {
  const Upstream = upstream_transport_constructors.get(upstream_mode);
  if (!Upstream) throw new Error(`unknown upstream_mode=${upstream_mode}`);
  this.upstream = new Upstream(upstream_config);
}
```
**Reason:** Allows the proxy to share a single upstream transport instance across multiple client connections, reducing the race condition between extension and downstream clients.

---

## Extension Changes

### `extension/manifest.json`

#### 1. Added WSS host permission (line 20)
```json
// Before:
"host_permissions": ["<all_urls>", "http://localhost/*", "http://127.0.0.1/*", "ws://localhost/*", "ws://127.0.0.1/*"]

// After:
"host_permissions": ["<all_urls>", "http://localhost/*", "http://127.0.0.1/*", "ws://localhost/*", "ws://127.0.0.1/*", "wss://penguin.linux.test/*", "http://penguin.linux.test/*"]
```
**Reason:** ChromeOS extensions use `penguin.linux.test` as the hostname for localhost forwarding, and require explicit host permissions for WSS connections.

---

### `extension/modcdp/service_worker.js`

This is a bundled/transpiled file. The key changes are:

#### 1. Changed default bridge URL (embedded in bundle)
```javascript
// Before:
var DEFAULT_REVERSE_BRIDGE_URL = "ws://127.0.0.1:29292";

// After:
var DEFAULT_REVERSE_BRIDGE_URL = "wss://penguin.linux.test:29292";
```
**Reason:** ChromeOS extensions require secure WebSocket (WSS) connections. The extension now connects to the proxy via the `penguin.linux.test` hostname which ChromeOS resolves to the Crostini container's localhost.

#### 2. Removed NATS and NativeMessaging transports (lines 31010-31018)
```javascript
// Before:
for (const transport of [
  new ReverseWSDownstreamTransport(),
  new NativeMessagingDownstreamTransport(),
  new NATSDownstreamTransport()
]) {
  this.downstream.add(transport);
}

// After:
for (const transport of [
  new ReverseWSDownstreamTransport()
  // NATSDownstreamTransport removed - we use ReverseWS mode
  // NativeMessagingDownstreamTransport removed - not available in ChromeOS
]) {
  this.downstream.add(transport);
}
```
**Reason:** 
- NATSDownstreamTransport would connect to `ws://127.0.0.1:4223` which had no server running
- NativeMessagingDownstreamTransport is not available in ChromeOS
- Only ReverseWSDownstreamTransport is needed for the Crostini use case

#### 3. Added debug error reporting (lines 30711-30720, 175-180)
```javascript
// Added to error handler:
console.error("[ModCDP Debug] NATS connection error");
fetch("http://127.0.0.1:9223/debug-report", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ type: "nats_error", url: endpoint, timestamp: Date.now() })
}).catch(() => {});

// Added to close handler:
console.error("[ModCDP Debug] NATS connection closed");
fetch("http://127.0.0.1:9223/debug-report", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ type: "nats_closed", url: endpoint, timestamp: Date.now() })
}).catch(() => {});
```
**Reason:** Reports connection errors to the proxy debug endpoint for visibility into extension-side issues.

---

### `js/src/transport/ReverseWSDownstreamTransport.ts`

#### 1. Changed default bridge URL (line 13)
```typescript
// Before:
const DEFAULT_REVERSE_BRIDGE_URL = "ws://127.0.0.1:29292";

// After:
const DEFAULT_REVERSE_BRIDGE_URL = "wss://penguin.linux.test:29292";
```
**Reason:** ChromeOS extensions require WSS connections; `penguin.linux.test` is the ChromeOS hostname for Crostini localhost forwarding.

#### 2. Added debug error reporting function (lines 15-29)
```typescript
// Debug helper: Report errors to help diagnose connection issues
function reportReverseWSError(type: string, url: string, error?: string) {
  try {
    console.log(`[ModCDP Debug] ${type}: ${url}`, error ? `Error: ${error}` : "");
    fetch("http://127.0.0.1:9223/debug-report", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ type, url, error, extension: "reversews", timestamp: Date.now() })
    }).catch(() => {});
  } catch {
    // Ignore errors in reporting
  }
}
```
**Reason:** Provides visibility into extension connection failures for debugging.

#### 3. Added connection attempt logging (line 97)
```typescript
console.log("[ModCDP] Attempting to connect to:", this.config.downstream_reversews_url);
```
**Reason:** Helps diagnose connection timing issues.

#### 4. Enhanced error/close event handlers (lines 186-198)
```typescript
ws.addEventListener("error", (event) => {
  console.error("ReverseWS error:", event?.type || "unknown", endpoint);
  reportReverseWSError("connection_failed", endpoint, event?.message || "unknown");
  if (this.socket === ws) this.socket = null;
  this.scheduleReconnect();
});
ws.addEventListener("close", (event) => {
  console.error("ReverseWS closed:", event?.code, event?.reason, endpoint);
  reportReverseWSError("connection_closed", endpoint, event?.reason || `code=${event?.code}`);
  if (this.socket === ws) this.socket = null;
  this.scheduleReconnect();
});
```
**Reason:** Detailed error logging for debugging WSS certificate and connection issues in ChromeOS.

---

## Configuration Notes

### Certificate Generation
Certificates should be generated via mkcert for ChromeOS compatibility:

```bash
mkcert -install
mkcert penguin.linux.test localhost 127.0.0.1
```

This creates `cert.pem` and `key.pem` files that the proxy uses to serve WSS on port 29292.

### Proxy Startup
```bash
npx tsx js/src/proxy/proxy.ts --upstream-mode=reversews --port 9223 --upstream-reversews-cert-dir=js/proxy
```

- **Port 9223:** CDP endpoint for Stagehand downstream clients
- **Port 29292:** WSS endpoint for ChromeOS extension connections

### Extension Reload Required
After making changes, users must reload the extension at `chrome://extensions`:
1. Enable Developer mode (toggle in top-right)
2. Click "Update" button to force reload from unpacked extension
3. OR: Remove extension entirely, then "Load unpacked" again

---

## Files Changed Summary

| File | Changes |
|------|---------|
| `js/src/proxy/proxy.ts` | DEFAULT_HOST change, shared upstream transport, debug endpoint |
| `js/src/transport/ReverseWSUpstreamTransport.ts` | WSS/HTTPS support, console logging |
| `js/src/client/ModCDPClient.ts` | `_existing_upstream` parameter |
| `js/src/transport/ReverseWSDownstreamTransport.ts` | WSS URL change, debug error reporting |
| `extension/manifest.json` | Added WSS host permissions |
| `extension/modcdp/service_worker.js` | Bundled changes (WSS URL, transport removal) |
| `js/proxy/extension-status.json` | New file for status reporting (generated at runtime) |

---

## Troubleshooting

### Certificate Mismatch
If Chrome reports "cert authority invalid":
- Ensure certificates include both `penguin.linux.test` and `localhost` SANs
- Regenerate certificates: `mkcert penguin.linux.test localhost 127.0.0.1`

### Connection Refused (ws://)
- Extension must use WSS (`wss://penguin.linux.test:29292`)
- Proxy must be started with `--upstream-reversews-cert-dir` pointing to certificate directory

### 4223 Error (NATS)
This error occurred when the extension tried to connect to NATS on port 4223. Fixed by removing NATSDownstreamTransport.

### Race Condition (CDP connection closed: code=1006)
- Extension must connect and send hello BEFORE downstream CDP client connects
- Shared upstream transport hack helps mitigate this, but proper extension reload is essential