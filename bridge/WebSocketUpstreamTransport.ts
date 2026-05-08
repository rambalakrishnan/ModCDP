import type { CdpCommandMessage } from "../types/modcdp.js";
import { resolveCdpWebSocketUrl } from "./BrowserLauncher.js";
import { UpstreamTransport, type UpstreamTransportConfig } from "./UpstreamTransport.js";

export class WebSocketUpstreamTransport extends UpstreamTransport {
  readonly mode = "ws" as const;
  readonly endpoint_kind = "raw_cdp" as const;
  ws: WebSocket | null = null;

  constructor(url: string | null = null) {
    super();
    this.url = url ?? "";
  }

  update(config: UpstreamTransportConfig = {}) {
    const url = config.ws_url ?? config.cdp_url ?? config.url;
    if (url) this.url = url;
    return this;
  }

  getServerConfig() {
    return this.url ? { loopback_cdp_url: this.url } : {};
  }

  async connect() {
    if (!this.url) throw new Error("upstream.mode=ws requires upstream.ws_url or launcher-provided ws_url.");
    this.url = await resolveCdpWebSocketUrl(this.url, "upstream.ws_url");
    const ws = new WebSocket(this.url);
    this.ws = ws;
    ws.addEventListener("message", (event) => this.parseAndEmitRecv(event.data));
    ws.addEventListener("close", () => this.emitClose(new Error("CDP websocket closed")));
    ws.addEventListener("error", () => this.emitClose(new Error("CDP websocket error")));
    await new Promise<void>((resolve, reject) => {
      const cleanup = () => {
        ws.removeEventListener("open", onOpen);
        ws.removeEventListener("error", onError);
      };
      const onOpen = () => {
        cleanup();
        resolve();
      };
      const onError = () => {
        cleanup();
        reject(new Error("CDP websocket error"));
      };
      ws.addEventListener("open", onOpen);
      ws.addEventListener("error", onError);
    });
  }

  send(message: CdpCommandMessage) {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) throw new Error("CDP websocket is not connected.");
    this.ws.send(JSON.stringify(message));
  }

  async close() {
    try {
      this.ws?.close();
    } catch {}
    this.ws = null;
  }
}
