import assert from "node:assert/strict";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test } from "vitest";

import { LocalBrowserLauncher } from "../bridge/LocalBrowserLauncher.js";
import { ModCDPClient } from "../client/js/ModCDPClient.js";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const EXTENSION_PATH = path.resolve(HERE, "..", "dist", "extension");

test("reversews upstream accepts a real extension reverse connection and routes CDP through loopback", async () => {
  const browser = new ModCDPClient({
    launch: {
      mode: "local",
      options: { headless: process.platform === "linux", sandbox: process.platform !== "linux" },
    },
    upstream: { mode: "ws" },
    extension: {
      mode: "auto",
      path: EXTENSION_PATH,
      service_worker_url_suffixes: ["/modcdp/service_worker.js"],
      trust_service_worker_target: true,
    },
  });
  let reverse: ModCDPClient | null = null;

  try {
    await browser.connect();
    const reverse_port = await LocalBrowserLauncher.freePort();
    const reverse_bind = `127.0.0.1:${reverse_port}`;
    const reverse_url = `ws://${reverse_bind}`;
    reverse = new ModCDPClient({
      launch: { mode: "none" },
      upstream: { mode: "reversews", reversews_bind: reverse_bind },
      extension: { mode: "none" },
      server: {
        loopback_cdp_url: browser.cdp_url,
        routes: { "*.*": "loopback_cdp" },
      },
    });
    const connect_promise = reverse.connect();
    await browser.send("Mod.evaluate", {
      expression: `globalThis.ModCDP.startReverseBridge(${JSON.stringify(reverse_url)}, { reconnect_interval_ms: 100 })`,
    });
    await connect_promise;
    assert.equal(reverse.transport?.mode, "reversews");
    assert.equal(reverse.upstream_endpoint_kind, "modcdp_server");
    assert.equal(reverse.transport?.url, reverse_url);
    const version = (await reverse.send("Browser.getVersion")) as Record<string, unknown>;
    assert.equal(typeof version.product, "string");
  } finally {
    await reverse?.close();
    await browser.close();
  }
}, 60_000);
