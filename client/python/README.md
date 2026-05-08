# ModCDP

ModCDP is a small Chrome DevTools Protocol client and extension bridge. It connects to Chrome over CDP, discovers or injects the ModCDP extension service worker, and routes normal CDP commands, custom commands, and custom events through the same client API.

This package contains the Python client. It mirrors the JavaScript and Go clients so SDKs can use the same option names and behavior across languages.

## Install

```sh
pip install modcdp
```

## Basic Usage

```python
from modcdp import ModCDPClient

cdp = ModCDPClient().connect()
version = cdp.send("Browser.getVersion")
print(version)
cdp.close()
```

When no `cdp_url` is provided, the client can launch a local browser and load the bundled ModCDP extension. The package includes the extension zip used for automatic injection.

## Options

The Python constructor mirrors the JavaScript and Go clients:

- `cdp_url`: upstream CDP HTTP or websocket URL.
- `extension_path`: extension directory or zip. Defaults to the bundled extension zip.
- `routes`: client-side route map such as `{ "Mod.*": "service_worker", "*.*": "direct_cdp" }`.
- `server`: service-worker server config, including `loopback_cdp_url` and `routes`.
- `custom_commands`: custom commands registered during connect.
- `custom_events`: custom events registered during connect.
- `custom_middlewares`: custom request/response/event middleware registered during connect.
- `service_worker_url_includes` and `service_worker_url_suffixes`: service-worker discovery filters.
- `scan_for_existing_localhost_9222`: attach to localhost Chrome before launching a new browser.
- `mirror_upstream_events`: mirror upstream CDP events through the ModCDP service worker.
- `*_timeout_ms` and `*_interval_ms`: override CDP send, websocket connect, service-worker probe, event wait, and polling timings.

## Custom Commands And Events

```python
from modcdp import ModCDPClient

cdp = ModCDPClient(
    custom_commands=[
        {
            "name": "Custom.echo",
            "expression": "(params) => ({ value: params.value })",
        }
    ],
    custom_events=[
        {
            "name": "Custom.ready",
        }
    ],
).connect()

print(cdp.send("Custom.echo", {"value": "hello"}))
cdp.close()
```

## Repository

Source, examples, JavaScript client, Go client, and extension implementation live at:

https://github.com/pirate/ModCDP
