declare global {
  var __MODCDP_RUNTIME_CONFIG__: Record<string, unknown> | undefined;
}

globalThis.__MODCDP_RUNTIME_CONFIG__ ??= {};

export {};
