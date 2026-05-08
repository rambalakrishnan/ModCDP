from __future__ import annotations

from typing import Any, TypedDict, cast


class BrowserLaunchOptions(TypedDict, total=False):
    executable_path: str | None
    port: int | None
    user_data_dir: str | None
    headless: bool
    sandbox: bool
    extra_args: list[str]
    chrome_ready_timeout_ms: int
    chrome_ready_poll_interval_ms: int


class LaunchedBrowser(TypedDict):
    cdp_url: str | None
    ws_url: str | None
    close: Any


DEFAULT_CHROME_READY_TIMEOUT_MS = 45_000
DEFAULT_CHROME_READY_POLL_INTERVAL_MS = 100


class BrowserLauncher:
    def __init__(self, options: BrowserLaunchOptions | None = None) -> None:
        self.options = cast(BrowserLaunchOptions, dict(options or {}))

    def launch(self, options: BrowserLaunchOptions | None = None) -> LaunchedBrowser:
        raise NotImplementedError(f"{type(self).__name__}.launch is not implemented.")
