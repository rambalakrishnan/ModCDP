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


if __name__ == "__main__":
    unittest.main()
