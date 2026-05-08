from __future__ import annotations

from .UpstreamTransport import UpstreamTransport


DEFAULT_NATIVE_MESSAGING_HOST_NAME = "com.modcdp.bridge"


class NativeMessagingUpstreamTransport(UpstreamTransport):
    mode = "nativemessaging"
    endpoint_kind = "modcdp_server"

    def __init__(self, manifest_path: str | None = None, host_name: str = DEFAULT_NATIVE_MESSAGING_HOST_NAME) -> None:
        super().__init__()
        self.manifest_path = manifest_path
        self.host_name = host_name

    def connect(self) -> None:
        raise NotImplementedError("upstream.mode='nativemessaging' is not implemented by the Python client yet.")
