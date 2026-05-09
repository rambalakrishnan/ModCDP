from __future__ import annotations

import sys
import unittest
from pathlib import Path

from modcdp import ModCDPClient
from modcdp.NativeMessagingUpstreamTransport import DEFAULT_NATIVE_MESSAGING_HOST_NAME, NativeMessagingUpstreamTransport


@unittest.skipIf(sys.platform.startswith("win"), "native messaging profile manifest path is not implemented on Windows")
class NativeMessagingUpstreamTransportTests(unittest.TestCase):
    def test_config_owns_manifest_host_wait_timeout_loopback_and_injector_config(self) -> None:
        transport = NativeMessagingUpstreamTransport(
            {
                "manifest_path": "/tmp/modcdp-native-host.json",
                "manifest_paths": ["/tmp/modcdp-native-host-extra.json"],
                "host_name": "com.modcdp.test",
                "extension_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
                "wait_timeout_ms": 10,
            }
        )
        self.assertEqual(transport.getInjectorConfig(), {"native_host_name": "com.modcdp.test"})
        self.assertEqual(transport.getServerConfig(), {})
        self.assertIs(
            transport.update(
                {
                    "ws_url": "ws://127.0.0.1:9222/devtools/browser/test",
                    "manifest_paths": [],
                    "native_host_name": "com.modcdp.updated",
                    "wait_timeout_ms": 5,
                }
            ),
            transport,
        )
        self.assertEqual(
            transport.getServerConfig(),
            {"loopback_cdp_url": "ws://127.0.0.1:9222/devtools/browser/test"},
        )
        self.assertEqual(transport.getInjectorConfig(), {"native_host_name": "com.modcdp.updated"})
        self.assertFalse(transport.include_default_manifest_paths)
        transport.update({"manifest_path": None})
        self.assertTrue(transport.include_default_manifest_paths)
        with self.assertRaisesRegex(RuntimeError, r"Timed out waiting 5ms for native messaging host com\.modcdp\.updated"):
            transport.waitForPeer()

    def test_installs_launch_profile_native_host_manifest_and_connects_to_real_extension(self) -> None:
        cdp = ModCDPClient(
            launch={"mode": "local", "options": {"headless": True, "sandbox": False}},
            upstream={"mode": "nativemessaging"},
            extension={
                "mode": "auto",
                "service_worker_url_suffixes": ["/modcdp/service_worker.js"],
                "trust_service_worker_target": True,
            },
            server={"routes": {"*.*": "loopback_cdp"}},
        )

        try:
            cdp.connect()
            self.assertEqual(cdp.transport.mode if cdp.transport else None, "nativemessaging")
            self.assertEqual(cdp.upstream_endpoint_kind, "modcdp_server")
            transport_url = cdp.transport.url if cdp.transport and cdp.transport.url else ""
            self.assertRegex(transport_url, r"^native://com\.modcdp\.bridge@127\.0\.0\.1:\d+$")
            profile_dir = cdp._launched_browser.get("profile_dir") if cdp._launched_browser else ""
            self.assertTrue(
                (Path(profile_dir) / "NativeMessagingHosts" / f"{DEFAULT_NATIVE_MESSAGING_HOST_NAME}.json").exists()
            )
            version = cdp.send("Browser.getVersion")
            self.assertIsInstance(version["product"], str)
        finally:
            cdp.close()


if __name__ == "__main__":
    unittest.main()
