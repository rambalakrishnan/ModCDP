from __future__ import annotations

import unittest

from modcdp.BrowserLauncher import BrowserLauncher


class BrowserLauncherTests(unittest.TestCase):
    def test_merges_launch_config_and_exposes_transport_and_injector_config(self) -> None:
        launcher = BrowserLauncher(
            {
                "cdp_url": "http://127.0.0.1:9222",
                "ws_url": "ws://127.0.0.1:9222/devtools/browser/initial",
                "user_data_dir": "/tmp/modcdp-browser-launcher",
                "browserbase_api_key": "test-key",
                "extension_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
                "extra_args": ["--load-extension=/tmp/one"],
            }
        )
        launcher.update(
            {
                "ws_url": "ws://127.0.0.1:9222/devtools/browser/updated",
                "extra_args": ["--load-extension=/tmp/two", "--window-size=900,700"],
            }
        )

        self.assertEqual(
            launcher.options.get("extra_args"),
            ["--window-size=900,700", "--load-extension=/tmp/one,/tmp/two"],
        )
        self.assertEqual(
            {
                "cdp_url": launcher.getTransportConfig()["cdp_url"],
                "ws_url": launcher.getTransportConfig()["ws_url"],
                "user_data_dir": launcher.getTransportConfig()["user_data_dir"],
            },
            {
                "cdp_url": "http://127.0.0.1:9222",
                "ws_url": "ws://127.0.0.1:9222/devtools/browser/updated",
                "user_data_dir": "/tmp/modcdp-browser-launcher",
            },
        )
        self.assertEqual(
            {
                "browserbase_api_key": launcher.getInjectorConfig()["browserbase_api_key"],
                "extension_id": launcher.getInjectorConfig()["extension_id"],
            },
            {
                "browserbase_api_key": "test-key",
                "extension_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
            },
        )
        with self.assertRaisesRegex(NotImplementedError, "BrowserLauncher.launch is not implemented"):
            launcher.launch()


if __name__ == "__main__":
    unittest.main()
