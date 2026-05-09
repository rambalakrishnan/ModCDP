from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from modcdp.ExtensionInjector import ExtensionInjector


class ExtensionInjectorTests(unittest.TestCase):
    def test_owns_shared_injector_config_and_runtime_transport_config(self) -> None:
        injector = ExtensionInjector(
            {
                "extension_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
                "service_worker_url_suffixes": ["/modcdp/service_worker.js"],
                "reverse_proxy_url": "ws://127.0.0.1:29292",
            }
        )
        injector.update({"native_host_name": "com.modcdp.bridge"})

        self.assertEqual(injector.getTransportConfig(), {"extension_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
        self.assertEqual(injector.getLauncherConfig(), {})
        self.assertTrue(
            injector.serviceWorkerTargetMatches(
                {
                    "targetId": "target-1",
                    "type": "service_worker",
                    "url": "chrome-extension://aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/modcdp/service_worker.js",
                }
            )
        )
        with tempfile.TemporaryDirectory() as extension_path:
            injector.writeExtensionRuntimeConfig(extension_path)
            self.assertEqual(
                (Path(extension_path) / "modcdp" / "config.json").read_text(),
                '{\n  "reverse_proxy_url": "ws://127.0.0.1:29292",\n  "native_host_name": "com.modcdp.bridge"\n}\n',
            )

        with self.assertRaisesRegex(NotImplementedError, "ExtensionInjector.inject is not implemented"):
            injector.inject()


if __name__ == "__main__":
    unittest.main()
