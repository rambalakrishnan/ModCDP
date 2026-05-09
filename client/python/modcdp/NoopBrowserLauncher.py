from __future__ import annotations

from .BrowserLauncher import BrowserLaunchOptions, BrowserLauncher, LaunchedBrowser


class NoopBrowserLauncher(BrowserLauncher):
    def launch(self, options: BrowserLaunchOptions | None = None) -> LaunchedBrowser:
        self.launched = {"cdp_url": None, "ws_url": None, "close": lambda: None}
        return self.launched
