import assert from "node:assert/strict";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test } from "vitest";

import { PipeUpstreamTransport } from "../bridge/PipeUpstreamTransport.js";
import { ModCDPClient } from "../client/js/ModCDPClient.js";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const EXTENSION_PATH = path.resolve(HERE, "..", "dist", "extension");

test("pipe upstream constructor, update, launcher config, and unconnected errors match the transport surface", async () => {
  const transport = new PipeUpstreamTransport(null, null, "pipe://constructor");
  assert.equal(transport.mode, "pipe");
  assert.equal(transport.endpoint_kind, "raw_cdp");
  assert.equal(transport.url, "pipe://constructor");
  assert.deepEqual(transport.getLauncherConfig(), { remote_debugging: "pipe" });
  assert.equal(transport.update({ cdp_url: "pipe://1234" }), transport);
  assert.equal(transport.url, "pipe://1234");
  await assert.rejects(() => transport.connect(), /upstream\.mode=pipe requires/);
  assert.throws(() => transport.send({ id: 1, method: "Browser.getVersion" }), /CDP pipe is not connected/);
});

test("pipe upstream launches a real browser and uses a pid-scoped pipe URL", async () => {
  const cdp = new ModCDPClient({
    launch: {
      mode: "local",
      options: { headless: true, sandbox: process.platform !== "linux" },
    },
    upstream: { mode: "pipe" },
    extension: {
      mode: "auto",
      path: EXTENSION_PATH,
      service_worker_url_suffixes: ["/modcdp/service_worker.js"],
      trust_service_worker_target: true,
    },
  });

  try {
    await cdp.connect();
    assert.equal(cdp.transport?.mode, "pipe");
    assert.equal(cdp.upstream_endpoint_kind, "raw_cdp");
    assert.match(cdp.cdp_url ?? "", /^pipe:\/\/\d+$/);
    assert.equal(cdp.transport?.url, cdp.cdp_url);
    const version = (await cdp.sendRaw("Browser.getVersion")) as Record<string, unknown>;
    assert.equal(typeof version.product, "string");
  } finally {
    await cdp.close();
  }
}, 60_000);
