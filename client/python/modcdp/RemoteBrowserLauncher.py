from __future__ import annotations

from .BrowserLauncher import BrowserLaunchOptions, BrowserLauncher, LaunchedBrowser


class RemoteBrowserLauncher(BrowserLauncher):
    def __init__(self, options: BrowserLaunchOptions | None = None, cdp_url: str | None = None) -> None:
        super().__init__(options)
        self.cdp_url = cdp_url

    def launch(self, options: BrowserLaunchOptions | None = None) -> LaunchedBrowser:
        if not self.cdp_url:
            raise RuntimeError("launch.mode='remote' requires upstream.ws_url.")
        return {"cdp_url": self.cdp_url, "ws_url": self.cdp_url, "close": lambda: None}
