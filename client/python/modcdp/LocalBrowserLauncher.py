from __future__ import annotations

import json
import glob
import os
import re
import subprocess
import sys
import tempfile
import time
import urllib.request
from pathlib import Path
from typing import cast

from .BrowserLauncher import (
    DEFAULT_CHROME_READY_POLL_INTERVAL_MS,
    DEFAULT_CHROME_READY_TIMEOUT_MS,
    BrowserLaunchOptions,
    BrowserLauncher,
    LaunchedBrowser,
)


class LocalBrowserLauncher(BrowserLauncher):
    @staticmethod
    def find_chrome_binary(explicit: str | None = None) -> str:
        candidates = [explicit, os.environ.get("CHROME_PATH"), *_candidate_paths()]
        for candidate in candidates:
            if candidate and Path(candidate).exists():
                return str(candidate)
        tried = ", ".join(str(candidate) for candidate in candidates if candidate)
        raise RuntimeError(f"No Chrome/Chromium binary found. Tried: {tried}. Set CHROME_PATH or pass executable_path.")

    def launch(self, options: BrowserLaunchOptions | None = None) -> LaunchedBrowser:
        merged = cast(BrowserLaunchOptions, {**self.options, **dict(options or {})})
        executable_path = self.find_chrome_binary(merged.get("executable_path"))
        port = int(merged.get("port") or _free_port())
        temp_profile_dir: tempfile.TemporaryDirectory[str] | None = None
        profile_dir = merged.get("user_data_dir")
        if not profile_dir:
            temp_profile_dir = tempfile.TemporaryDirectory(prefix="modcdp.")
            profile_dir = temp_profile_dir.name
        args = [
            "--enable-unsafe-extension-debugging",
            "--remote-allow-origins=*",
            "--no-first-run",
            "--no-default-browser-check",
            "--disable-default-apps",
            "--disable-dev-shm-usage",
            "--disable-background-networking",
            "--disable-backgrounding-occluded-windows",
            "--disable-renderer-backgrounding",
            "--disable-background-timer-throttling",
            "--disable-sync",
            "--disable-features=DisableLoadExtensionCommandLineSwitch",
            "--password-store=basic",
            "--use-mock-keychain",
            "--disable-gpu",
            f"--user-data-dir={profile_dir}",
            "--remote-debugging-address=127.0.0.1",
            f"--remote-debugging-port={port}",
        ]
        default_headless = sys.platform.startswith("linux") and not os.environ.get("DISPLAY")
        if merged.get("headless", default_headless):
            args.append("--headless=new")
        if merged.get("sandbox", False) is False:
            args.append("--no-sandbox")
        args.extend(list(merged.get("extra_args") or []))
        args.append("about:blank")
        process = subprocess.Popen([executable_path, *args], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        cdp_url = f"http://127.0.0.1:{port}"
        timeout_s = int(merged.get("chrome_ready_timeout_ms") or DEFAULT_CHROME_READY_TIMEOUT_MS) / 1000
        poll_s = int(merged.get("chrome_ready_poll_interval_ms") or DEFAULT_CHROME_READY_POLL_INTERVAL_MS) / 1000
        deadline = time.time() + timeout_s
        while time.time() < deadline:
            try:
                with urllib.request.urlopen(f"{cdp_url}/json/version", timeout=0.5) as response:
                    version = json.loads(response.read())
                    self.launched = {
                        "cdp_url": cdp_url,
                        "ws_url": version.get("webSocketDebuggerUrl") or cdp_url,
                        "profile_dir": profile_dir,
                        "close": lambda: _close(process, temp_profile_dir),
                    }
                    return self.launched
            except Exception:
                time.sleep(poll_s)
        _close(process, temp_profile_dir)
        raise RuntimeError(f"Chrome at {cdp_url} did not become ready within {timeout_s}s")


def _newest_first(candidates: list[str]) -> list[str]:
    def score(candidate: str) -> tuple[int, float, str]:
        numbers = [int(part) for part in re.findall(r"\d+", candidate)]
        version = max(numbers) if numbers else 0
        try:
            mtime = Path(candidate).stat().st_mtime
        except OSError:
            mtime = 0.0
        return (-version, -mtime, candidate)

    return sorted(dict.fromkeys(candidates), key=score)


def _chrome_for_testing_candidates() -> list[str]:
    home = Path.home()
    if sys.platform == "darwin":
        patterns = [
            str(home / "Library/Caches/ms-playwright/chromium-*/chrome-mac*/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing"),
            str(home / "Library/Caches/ms-playwright/chromium-*/chrome-mac*/Chromium.app/Contents/MacOS/Chromium"),
            str(home / "Library/Caches/puppeteer/chrome/mac*-*/chrome-mac*/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing"),
        ]
    elif sys.platform.startswith("win"):
        local_app_data = Path(os.environ.get("LOCALAPPDATA") or home / "AppData/Local")
        patterns = [
            str(local_app_data / "ms-playwright/chromium-*/chrome-win*/chrome.exe"),
            str(home / ".cache/puppeteer/chrome/win*-*/chrome-win*/chrome.exe"),
        ]
    else:
        patterns = [
            str(home / ".cache/ms-playwright/chromium-*/chrome-linux*/chrome"),
            "/opt/pw-browsers/chromium-*/chrome-linux*/chrome",
            str(home / ".cache/puppeteer/chrome/linux-*/chrome-linux*/chrome"),
        ]
    return _newest_first([match for pattern in patterns for match in glob.glob(pattern)])


def _candidate_paths() -> list[str]:
    home = Path.home()
    program_files = [value for value in [os.environ.get("PROGRAMFILES"), os.environ.get("PROGRAMFILES(X86)")] if value]
    if sys.platform == "darwin":
        canary = ["/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary"]
        stock = ["/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"]
    elif sys.platform.startswith("win"):
        local_app_data = Path(os.environ.get("LOCALAPPDATA") or home / "AppData/Local")
        canary = [
            str(local_app_data / "Google/Chrome SxS/Application/chrome.exe"),
            *[str(Path(base) / "Google/Chrome SxS/Application/chrome.exe") for base in program_files],
        ]
        stock = [
            *[str(Path(base) / "Google/Chrome/Application/chrome.exe") for base in program_files],
            str(local_app_data / "Google/Chrome/Application/chrome.exe"),
        ]
    else:
        canary = ["/usr/bin/google-chrome-canary", "/usr/bin/google-chrome-unstable", "/opt/google/chrome-unstable/chrome"]
        stock = ["/usr/bin/google-chrome-stable", "/usr/bin/google-chrome", "/opt/google/chrome/chrome"]
    return [*_chrome_for_testing_candidates(), *canary, *stock]


def _close(process: subprocess.Popen[bytes], temp_profile_dir: tempfile.TemporaryDirectory[str] | None) -> None:
    process.terminate()
    try:
        process.wait(timeout=2)
    except subprocess.TimeoutExpired:
        process.kill()
        process.wait(timeout=2)
    if temp_profile_dir:
        temp_profile_dir.cleanup()


def _free_port() -> int:
    import socket

    sock = socket.socket()
    sock.bind(("127.0.0.1", 0))
    try:
        return int(sock.getsockname()[1])
    finally:
        sock.close()
