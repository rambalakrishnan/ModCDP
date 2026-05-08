import { describe, expect, it } from "vitest";

import { LocalBrowserLauncher } from "../bridge/LocalBrowserLauncher.js";
import { RemoteBrowserLauncher } from "../bridge/RemoteBrowserLauncher.js";
import { CdpSocket, expectCdpBrowserSurface } from "./helpers.BrowserLauncher.js";

const LIVE_BROWSER_TIMEOUT_MS = 60_000;

describe("RemoteBrowserLauncher", () => {
  it(
    "connects to a real browser from both HTTP discovery and websocket CDP endpoints",
    { timeout: LIVE_BROWSER_TIMEOUT_MS },
    async () => {
      const local = await new LocalBrowserLauncher().launch({
        headless: true,
        sandbox: process.platform !== "linux",
        chrome_ready_timeout_ms: 45_000,
      });
      let cdp: CdpSocket | null = null;

      try {
        const fromHttp = await new RemoteBrowserLauncher({}, local.cdp_url).launch();
        expect(fromHttp.cdp_url).toBe(local.cdp_url);
        expect(fromHttp.ws_url).toBe(local.ws_url);
        cdp = await CdpSocket.connect(fromHttp.ws_url!);
        await expectCdpBrowserSurface(cdp);
        await fromHttp.close();

        const fromWs = await new RemoteBrowserLauncher().launch({ ws_url: local.ws_url });
        expect(fromWs.cdp_url).toBe(local.ws_url);
        expect(fromWs.ws_url).toBe(local.ws_url);
        await expectCdpBrowserSurface(cdp);
        await fromWs.close();
      } finally {
        await cdp?.close();
        await local.close();
      }
    },
  );
});
