from __future__ import annotations

import json
import unittest

from websocket import create_connection

from modcdp.LocalBrowserLauncher import LocalBrowserLauncher


class LocalBrowserLauncherTests(unittest.TestCase):
    def test_launches_real_browser_and_speaks_cdp(self) -> None:
        chrome = LocalBrowserLauncher(
            {
                "headless": True,
                "sandbox": False,
                "chrome_ready_timeout_ms": 45_000,
            }
        ).launch()
        ws_url = chrome["ws_url"]
        if ws_url is None:
            raise AssertionError("expected launcher to return ws_url")
        ws = create_connection(ws_url, timeout=10)

        try:
            ws.send(json.dumps({"id": 1, "method": "Browser.getVersion", "params": {}}))
            version = json.loads(ws.recv())
            self.assertEqual(version["id"], 1)
            self.assertIn("Chrome", version["result"]["product"])
            self.assertIsInstance(version["result"]["protocolVersion"], str)
        finally:
            ws.close()
            chrome["close"]()

    def test_launches_real_browser_over_remote_debugging_pipe(self) -> None:
        chrome = LocalBrowserLauncher(
            {
                "headless": True,
                "sandbox": False,
                "remote_debugging": "pipe",
                "chrome_ready_timeout_ms": 45_000,
            }
        ).launch()
        pipe_read = chrome.get("pipe_read")
        pipe_write = chrome.get("pipe_write")
        if pipe_read is None or pipe_write is None:
            raise AssertionError("expected launcher to return pipe handles")

        try:
            self.assertRegex(chrome["cdp_url"] or "", r"^pipe://\d+$")
            self.assertIsNone(chrome["ws_url"])
            pipe_write.write(json.dumps({"id": 10, "method": "Browser.getVersion", "params": {}}).encode() + b"\0")
            pipe_write.flush()
            response = _read_pipe_message(pipe_read)
            self.assertEqual(response["id"], 10)
            self.assertIn("Chrome", response["result"]["product"])
        finally:
            chrome["close"]()


def _read_pipe_message(pipe_read) -> dict:
    buffer = b""
    while True:
        chunk = pipe_read.read(1)
        if not chunk:
            raise AssertionError("pipe closed before CDP response")
        buffer += chunk
        if b"\0" not in buffer:
            continue
        raw, _ = buffer.split(b"\0", 1)
        return json.loads(raw.decode())


if __name__ == "__main__":
    unittest.main()
