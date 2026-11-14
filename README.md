# Vessel HQ extension SDK

Independent workspace for the Go gRPC SDK, public protobuf contract, native UI types and simple example plugins. The UniFi-style Stack Network Overview extension stays in Vessel HQ.

## Current distribution identity

The current prerelease is `github.com/TheModCraft-TMC/Vessel-HQ-Extension-SDK@v0.1.0-beta.2`, published from this repository using the root tag `v0.1.0-beta.2`. The maintainer authorized the independent module path during issue #299. The historical `github.com/TheModCraft-TMC/Vessel-HQ/sdk/go@v0.1.0-beta.1` remains immutable and does not gain this release's RPCs. Vessel consumes the checksum-verified published module without a sibling checkout. Joint validation of unpublished changes may use an explicit temporary workspace; never commit a machine-specific replacement.

Install the published prerelease:

```sh
go get github.com/TheModCraft-TMC/Vessel-HQ-Extension-SDK@v0.1.0-beta.2
```

Add your SDK imports before `go mod tidy`. See [the Go SDK guide](docs/go-sdk.md) for API examples, mTLS identities, scoped read permissions, extension-owned note writes, conflicts and Unix socket transport. Go 1.25 or newer, Node.js 24.21.0 and system tar are required.

## Layout

- Root Go module: SDK and independent real gRPC/mTLS tests.
- `proto/v1`: schema and generated public wire contract, including host-private RPC methods.
- `ui/sdk.ts`: native UI host contract; type-only imports are compiled by the Vessel host.
- `examples/environment-summary`: environment metadata read CLI.
- `examples/operator-client`: operator CLI and private short-lived development certificate helper.
- `examples/persistent-notes`: note read/write CLI using the same persisted note as the UI.
- `plugins/environment-summary`: minimal native UI plugin.
- `plugins/environment-summary-service`: package template combining simple UI and the Go read CLI.
- `plugins/persistent-notes`: native UI write-back plugin.

## Verify and build

```sh
GOWORK=off go test -race ./...
GOWORK=off go vet ./...
GOWORK=off go build ./...
# Pinned protobuf generators are documented in docs/go-sdk.md.
./generate.sh --check
node --test scripts/package-source.test.mjs
node scripts/package-source.mjs --out /tmp/vessel-extension-sdk.tar.gz
```

Simple UI plugins compile inside the trusted Vessel console, rather than running an independent web server. Their type-only `@/core/native-extensions/sdk` imports bind to the host contract. Keep `ui/sdk.ts` aligned with the host when changing that contract. Package them using the host's policy-enforcing builder:

```sh
node scripts/package-plugin.mjs --vessel-root /path/to/Vessel-HQ --plugin persistent-notes --out /tmp/persistent-notes.vesselhq
node scripts/package-plugin.mjs --vessel-root /path/to/Vessel-HQ --plugin environment-summary --out /tmp/environment-summary.vesselhq
node scripts/package-plugin.mjs --vessel-root /path/to/Vessel-HQ --plugin environment-summary-service --arch arm64 --out /tmp/environment-summary-service.vesselhq
```

The helper builds backend executables with this standalone SDK, recalculates the package digest, validates source policy and packages into a new destination. It does not edit source manifests, upload, grant permissions, launch services or modify the host. `--arch` accepts `amd64` or `arm64`, default `amd64`. Native UI remains trusted administrator code.

## Runtime limits

Use one shared outbound mTLS TCP gateway or a private shared Unix socket; no published port is required per stack, and extension containers never need the Docker socket. Both transports retain package-bound TLS 1.3 identity and fresh authorization on every RPC. Typed environment reads, extension-owned note compare-and-swap writes and separately granted settings reads are implemented. Settings secrets require an additional explicit machine grant. See [beta.2 settings](docs/settings-beta.2.md). Infrastructure mutations, managed service startup/reverse routing and production certificate lifecycle remain host work.

[Vessel HQ host and operator documentation](https://github.com/TheModCraft-TMC/Vessel-HQ/blob/feature/native-extensions-prerelease/docs/native-extensions.md).
