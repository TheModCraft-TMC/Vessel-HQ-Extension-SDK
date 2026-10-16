# Simple UI with a Go service

Package this template from the SDK workspace using:

```sh
node scripts/package-plugin.mjs --vessel-root /path/to/Vessel-HQ --plugin environment-summary-service --arch amd64 --out /tmp/environment-summary-service.vesselhq
```

This builds `examples/environment-summary` as a real Linux executable and includes the minimal environment-summary UI. It uses `VESSEL_EXTENSION_CLIENT_*` variables and `--environment-id 1` as described in [the SDK guide](../../docs/go-sdk.md). Review environment scope before packaging. Upload alone does not start the executable; configure the runtime and package-bound identity separately. No per-stack published ports or Docker socket mounts are required.
