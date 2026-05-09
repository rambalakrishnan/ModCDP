import assert from "node:assert/strict";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test } from "vitest";

import { LocalBrowserLauncher } from "../bridge/LocalBrowserLauncher.js";
import { startProxy } from "../bridge/proxy.js";
import { CdpSocket } from "./helpers.BrowserLauncher.js";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const EXTENSION_PATH = path.resolve(HERE, "..", "dist", "extension");

test("proxy upgrades a vanilla CDP websocket to ModCDP against a real browser", async () => {
  const proxy_port = await LocalBrowserLauncher.freePort();
  const proxy = await startProxy({
    port: proxy_port,
    launch: {
      mode: "local",
    },
    upstream: { mode: "ws" },
    extension: {
      mode: "auto",
      path: EXTENSION_PATH,
    },
    server: {
      routes: { "*.*": "loopback_cdp" },
    },
  });
  const cdp = await CdpSocket.connect(proxy.url);
  let target_id: string | null = null;

  try {
    const version = await cdp.send("Browser.getVersion");
    assert.equal(typeof version.product, "string");

    const evaluated = await cdp.send("Mod.evaluate", {
      expression: "({ ok: true, transport: 'proxy' })",
    });
    assert.deepEqual(evaluated, { ok: true, transport: "proxy" });

    const created = await cdp.send("Target.createTarget", { url: "about:blank#modcdp-proxy-test" });
    assert.equal(typeof created.targetId, "string");
    target_id = created.targetId as string;
  } finally {
    if (target_id) await cdp.send("Target.closeTarget", { targetId: target_id }).catch(() => ({}));
    await cdp.close();
    await proxy.close();
  }
}, 60_000);
