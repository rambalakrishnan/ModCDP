from __future__ import annotations

from .BrowserLauncher import BrowserLaunchOptions, BrowserLauncher, LaunchedBrowser


class BrowserbaseBrowserLauncher(BrowserLauncher):
    def launch(self, options: BrowserLaunchOptions | None = None) -> LaunchedBrowser:
        raise NotImplementedError("launch.mode='bb' is not implemented yet.")
