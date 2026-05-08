import type { BrowserLaunchOptions } from "./BrowserLauncher.js";
import { ExtensionInjector } from "./ExtensionInjector.js";

export class LocalBrowserLaunchExtensionInjector extends ExtensionInjector {
  private unpacked_extension_path: string | null = null;
  private cleanup: (() => Promise<void>) | null = null;

  async prepare() {
    const extension_path = this.options.extension_path;
    if (!extension_path) {
      await super.prepare();
      return;
    }
    if (this.unpacked_extension_path) {
      await super.prepare();
      return;
    }
    if (!extension_path.endsWith(".zip")) {
      this.unpacked_extension_path = extension_path;
      await super.prepare();
      return;
    }
    const [{ execFileSync }, fs, os, path] = await Promise.all([
      import("node:child_process"),
      import("node:fs"),
      import("node:os"),
      import("node:path"),
    ]);
    const unpacked_path = fs.mkdtempSync(path.join(os.tmpdir(), "modcdp-extension-"));
    execFileSync("unzip", ["-q", extension_path, "-d", unpacked_path]);
    this.unpacked_extension_path = unpacked_path;
    this.cleanup = async () => fs.rmSync(unpacked_path, { recursive: true, force: true });
    await super.prepare();
  }

  getLauncherConfig(): BrowserLaunchOptions {
    const extension_path = this.unpacked_extension_path;
    if (!extension_path) return {};
    const existing_args = [] as string[];
    if (!existing_args.some((arg) => arg.startsWith("--load-extension="))) {
      existing_args.push(`--load-extension=${extension_path}`);
    }
    return { extra_args: existing_args };
  }

  async inject() {
    const timeout_ms = Math.min(this.options.service_worker_probe_timeout_ms ?? 10_000, 3_000);
    const discovered = await this.waitForReadyServiceWorker(timeout_ms, {
      matched_only: this.options.trust_matched_service_worker,
    });
    return discovered ? { ...discovered, source: "local_launch" } : null;
  }

  async close() {
    await super.close();
    await this.cleanup?.();
    this.cleanup = null;
  }
}
