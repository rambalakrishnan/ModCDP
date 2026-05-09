from __future__ import annotations

import unittest

from modcdp import ModCDPClient


class PipeUpstreamTransportTests(unittest.TestCase):
    def test_launches_real_browser_and_uses_pid_scoped_pipe_url(self) -> None:
        cdp = ModCDPClient(
            launch={"mode": "local", "options": {"headless": True, "sandbox": False}},
            upstream={"mode": "pipe"},
            extension={
                "mode": "auto",
                "service_worker_url_suffixes": ["/modcdp/service_worker.js"],
                "trust_service_worker_target": True,
            },
        )

        try:
            cdp.connect()
            self.assertEqual(cdp.transport.mode if cdp.transport else None, "pipe")
            self.assertEqual(cdp.upstream_endpoint_kind, "raw_cdp")
            self.assertRegex(cdp.cdp_url or "", r"^pipe://\d+$")
            self.assertEqual(cdp.transport.url if cdp.transport else None, cdp.cdp_url)
            version = cdp.sendRaw("Browser.getVersion")
            self.assertIsInstance(version["product"], str)
        finally:
            cdp.close()


if __name__ == "__main__":
    unittest.main()
