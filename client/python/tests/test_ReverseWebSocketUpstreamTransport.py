from __future__ import annotations

import socket
import unittest

from modcdp import ModCDPClient


class ReverseWebSocketUpstreamTransportTests(unittest.TestCase):
    def test_accepts_real_extension_reverse_connection_and_routes_cdp_through_loopback(self) -> None:
        reverse_port = _free_port()
        reverse_bind = f"127.0.0.1:{reverse_port}"
        cdp = ModCDPClient(
            launch={"mode": "local", "options": {"headless": True, "sandbox": False}},
            upstream={"mode": "reversews", "reversews_bind": reverse_bind},
            extension={
                "mode": "auto",
                "service_worker_url_suffixes": ["/modcdp/service_worker.js"],
                "trust_service_worker_target": True,
            },
            server={"routes": {"*.*": "loopback_cdp"}},
        )

        try:
            cdp.connect()
            self.assertEqual(cdp.transport.mode if cdp.transport else None, "reversews")
            self.assertEqual(cdp.upstream_endpoint_kind, "modcdp_server")
            version = cdp.send("Browser.getVersion")
            self.assertIsInstance(version["product"], str)
        finally:
            cdp.close()


def _free_port() -> int:
    sock = socket.socket()
    sock.bind(("127.0.0.1", 0))
    try:
        return int(sock.getsockname()[1])
    finally:
        sock.close()


if __name__ == "__main__":
    unittest.main()
