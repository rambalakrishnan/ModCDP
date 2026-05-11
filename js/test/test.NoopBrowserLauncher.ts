import { describe, expect, it } from "vitest";

import { NoopBrowserLauncher } from "../src/launcher/NoopBrowserLauncher.js";

describe("NoopBrowserLauncher", () => {
  it("uses the real no-browser lifecycle and returns no CDP endpoints", async () => {
    const browser = await new NoopBrowserLauncher({
      cdp_url: "ws://127.0.0.1:1/devtools/browser/not-used",
      user_data_dir: "/tmp/not-used-by-noop",
    }).launch();

    expect(browser).toMatchObject({
      cdp_url: null,
    });
    expect(browser.proc).toBeUndefined();
    expect(browser.pipe_read).toBeUndefined();
    expect(browser.pipe_write).toBeUndefined();
    await browser.close();
    await browser.close();
  });
});
