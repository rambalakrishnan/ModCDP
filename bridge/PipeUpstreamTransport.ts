import type { CdpCommandMessage } from "../types/modcdp.js";
import { UpstreamTransport, type UpstreamTransportConfig } from "./UpstreamTransport.js";

export class PipeUpstreamTransport extends UpstreamTransport {
  readonly mode = "pipe" as const;
  readonly endpoint_kind = "raw_cdp" as const;
  declare url: string;
  private buffer = "";
  private connected = false;

  private pipe_read: NodeJS.ReadableStream | null;
  private pipe_write: NodeJS.WritableStream | null;

  constructor(
    pipe_read: NodeJS.ReadableStream | null = null,
    pipe_write: NodeJS.WritableStream | null = null,
    url = "pipe://unknown",
  ) {
    super();
    this.pipe_read = pipe_read;
    this.pipe_write = pipe_write;
    this.url = url;
  }

  update(config: UpstreamTransportConfig = {}) {
    this.pipe_read = config.pipe_read ?? this.pipe_read;
    this.pipe_write = config.pipe_write ?? this.pipe_write;
    this.url = config.cdp_url ?? config.url ?? this.url;
    return this;
  }

  getLauncherConfig() {
    return { remote_debugging: "pipe" as const };
  }

  async connect() {
    if (!this.pipe_read || !this.pipe_write) {
      throw new Error("upstream.mode=pipe requires launcher-provided remote-debugging pipe handles.");
    }
    if (this.connected) return;
    this.connected = true;
    this.pipe_read.on("data", (chunk) => this.read(chunk));
    this.pipe_read.on("end", () => this.emitClose(new Error("CDP pipe closed")));
    this.pipe_read.on("error", () => this.emitClose(new Error("CDP pipe error")));
    this.pipe_write.on("error", () => this.emitClose(new Error("CDP pipe write error")));
  }

  send(message: CdpCommandMessage) {
    if (!this.pipe_write || !this.connected) throw new Error("CDP pipe is not connected.");
    this.pipe_write.write(`${JSON.stringify(message)}\0`);
  }

  async close() {
    try {
      this.pipe_write?.end();
    } catch {}
    try {
      (this.pipe_read as { destroy?: () => void } | null)?.destroy?.();
    } catch {}
    this.connected = false;
  }

  private read(chunk: Buffer | string) {
    this.buffer += Buffer.isBuffer(chunk) ? chunk.toString("utf8") : chunk;
    for (;;) {
      const end = this.buffer.indexOf("\0");
      if (end < 0) return;
      const message = this.buffer.slice(0, end);
      this.buffer = this.buffer.slice(end + 1);
      if (message) this.parseAndEmitRecv(message);
    }
  }
}
