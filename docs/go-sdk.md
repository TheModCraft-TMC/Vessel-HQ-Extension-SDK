# Vessel HQ Go extension SDK

Experimental SDK for independent extension services. Module:
`github.com/TheModCraft-TMC/Vessel-HQ/sdk/go`; package name: `extension`.
Requires Go 1.25 or newer. It depends on gRPC/protobuf and their transport
dependencies, with no dependency on Vessel's backend, Docker or Kubernetes.

## Install the prerelease

The SDK has its own module version, separate from the Vessel application version:

```sh
go get github.com/TheModCraft-TMC/Vessel-HQ/sdk/go@v0.1.0-beta.1
```

The existing immutable Vessel repository tag is `sdk/go/v0.1.0-beta.1`. Its GitHub prerelease assets also contain
the public SDK source archive and Linux amd64/arm64 example executables. The SDK
is experimental; review the capability and lifecycle limits below before use.
See the [prerelease guide](https://github.com/TheModCraft-TMC/Vessel-HQ/blob/feature/native-extensions-prerelease/docs/releases/native-extensions-prerelease.md).

## Develop against a checkout

To test unpublished changes in a separate extension project:

```sh
go mod init example.com/my-vessel-extension
go mod edit -require=github.com/TheModCraft-TMC/Vessel-HQ/sdk/go@v0.0.0
go mod edit -replace=github.com/TheModCraft-TMC/Vessel-HQ/sdk/go=/absolute/path/to/vesselhq-extension-sdk
go mod tidy
```

Add the import and code before running `go mod tidy`, so Go retains the dependency.
The already published beta.1 uses the original Vessel submodule tag. A future standalone release needs an explicitly configured canonical remote, reviewed module identity and corresponding tag scheme; it must not inherit the old subdirectory tag scheme automatically. Vessel's `RELEASE_VERSION` remains the application version. There is no need to import the full backend module.

```go
package main

import (
    "context"
    "log"

    extension "github.com/TheModCraft-TMC/Vessel-HQ/sdk/go"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

func main() {
    config, err := extension.ConfigFromEnvironment()
    if err != nil { log.Fatal(err) }
    ctx := context.Background()
    client, err := extension.Dial(ctx, config)
    if err != nil { log.Fatal(err) }
    defer client.Close()

    environment, err := client.GetEnvironment(ctx, 1)
    if status.Code(err) == codes.PermissionDenied {
        log.Print("Environment access is not approved")
        return
    }
    if err != nil { log.Print(err); return }
    log.Printf("Environment %d: %s", environment.GetId(), environment.GetName())
}
```

The host operator supplies an issued client certificate with exactly one URI SAN:
`spiffe://vessel.local/extensions/<extension-id>/packages/<sha256-hex>`.
Its ID and digest must match the currently approved registration. The host must
approve `environments.read` for the requested environment and separately enable
unattended service reads. The SDK cannot create grants or provision production
identities. Development certificate instructions are in the
[operator example](../examples/operator-client/README.md).

Set these variables for the standalone example:

```sh
export VESSEL_EXTENSION_CLIENT_ADDRESS=127.0.0.1:19444
export VESSEL_EXTENSION_CLIENT_CA=/private/path/ca.crt
export VESSEL_EXTENSION_CLIENT_CERT=/private/path/client.crt
export VESSEL_EXTENSION_CLIENT_KEY=/private/path/client.key
# Optional: SERVER_NAME verifies a DNS SAN when connecting by IP.
# export VESSEL_EXTENSION_CLIENT_SERVER_NAME=vessel.example.internal
export VESSEL_EXTENSION_CLIENT_RPC_TIMEOUT=10s
cd /absolute/path/to/vesselhq-extension-sdk
GOWORK=off go run ./examples/environment-summary --environment-id 1
```

The environment variables configure the client, separate from the host's
`VESSEL_EXTENSION_GRPC_*` listener variables. You can instead pass `extension.Config`
directly to `Dial`. `Address` accepts `host:port` (brackets for IPv6) or an explicit
`unix:///absolute/clean/socket-path`. Other URLs and resolver schemes are rejected.

## No published service ports

Extensions do not need a published port on every stack. A service makes an
outbound connection to one Vessel gateway. On a private container network, use
the Vessel service name and its internal gateway port; omit Docker `ports:`.
The host retains the Docker socket and applies environment and capability grants.
Do not mount the Docker socket into an extension container.

For a same-host executable or containers sharing a dedicated private volume, the
gateway can instead listen on a Unix-domain socket:

```sh
# Host: create an operator-owned directory accessible only to the runtime UID.
mkdir -m 700 /private/runtime/vessel-extensions
export VESSEL_EXTENSION_GRPC_ADDRESS=unix:///private/runtime/vessel-extensions/gateway.sock
# Keep VESSEL_EXTENSION_GRPC_CERT, _KEY and _CLIENT_CA configured as usual.

# Extension: use the mounted socket path and the server certificate's DNS SAN.
export VESSEL_EXTENSION_CLIENT_ADDRESS=unix:///private/runtime/vessel-extensions/gateway.sock
export VESSEL_EXTENSION_CLIENT_SERVER_NAME=vessel.extensions.internal
# Keep VESSEL_EXTENSION_CLIENT_CA, _CERT and _KEY configured as usual.
```

Unix transport still requires mutually verified TLS 1.3 and package-bound client
identity. `SERVER_NAME` is mandatory because a filesystem path cannot identify the
server certificate. The host requires an existing private parent directory,
creates the socket with mode `0600`, refuses any existing socket/file/symlink at
that path, and removes its socket on orderly shutdown. Run both sides with the
same runtime UID; never broaden socket permissions or share the volume with
unrelated workloads. After an unclean shutdown, the operator must inspect and
remove a stale socket before restarting. The SDK does not start containers or
implement reverse service routing. Current RPC capabilities are unchanged by
choosing this transport.

## Connection and errors

- `Dial(ctx, config)` loads the dedicated CA and client identity, requires TLS 1.3,
  verifies the server name and fetches registration before returning a client.
  The registration must match the certificate's package identity. Failed setup
  closes the connection. There is no plaintext or skip-verification option.
- Reuse one client concurrently, then `Close()` it on shutdown. Close is
  idempotent. gRPC handles connection reuse and reconnects; the SDK does not replay
  application calls or cache grants. Recreate the client to rotate certificates.
- Every RPC, including initial registration, is bounded by `RPCTimeout` (default
  ten seconds). An earlier context deadline or cancellation takes precedence.
  Errors wrap their cause and preserve `status.Code(err)`.
- `GetRegistration(ctx)` returns fresh approval diagnostics.
  `GetEnvironment(ctx, id)` returns ID/name/type/status only; no infrastructure
  URLs, credentials, snapshots or engine sockets. The host rechecks current grants
  on each call, including a concurrent revocation check before returning data.
- Requests are bounded to 64 KiB and responses to 256 KiB. Responses for a
  different registration/package or environment fail with `DataLoss`.

## Extension-owned notes

`GetNote(ctx, revision)` and `PutNote(ctx, revision, note)` access the extension's
own persisted text note. The administrator must explicitly grant
`storage.service.read` and/or `storage.service.write` in the package approval.
These are separate from native UI `storage.read`/`storage.write`; they require no
UI trust or environment metadata-read toggle. A write grant does not imply read.
The certificate binds calls to the approved package identity, and every call
checks current grants and approval revision inside the datastore transaction.

```go
registration, err := client.GetRegistration(ctx)
if err != nil { return err }
note, err := client.GetNote(ctx, registration.GetRevision())
if err != nil { return err }
note.Text = "Updated by the extension service"
saved, err := client.PutNote(ctx, registration.GetRevision(), note)
_ = saved
```

Notes accept at most 16 KiB of UTF-8 text. The initial note has version zero.
Writes compare the supplied note version and increment it atomically; concurrent
changes return `codes.Aborted` so callers can reload and resolve the conflict.
Changed approval revisions return `codes.FailedPrecondition`; revoked grants or
replaced packages return `codes.PermissionDenied`. Persistence and metadata-only
audit commit together, and note text never enters the audit payload. This is the
same note storage used by the native UI, not a secret store or Docker mutation.
The SDK does not retry writes automatically.

The standalone CLI uses the same `VESSEL_EXTENSION_CLIENT_*` variables for TCP or
Unix transport. It needs the machine read grant; updates also need the independent
machine write grant:

```sh
# Read metadata only: {"version":3,"saved":false}
GOWORK=off go run ./examples/persistent-notes
# Save once using the current approval revision and note version.
GOWORK=off go run ./examples/persistent-notes --text='A service-owned note'
# An explicitly supplied empty string clears the note; omitting --text never writes.
GOWORK=off go run ./examples/persistent-notes --text=''
# Include note text only when explicitly requested.
GOWORK=off go run ./examples/persistent-notes --show-text
```

Default output contains only `version` and `saved`; note text is not logged. A
conflict exits with an error and does not retry. A container running this CLI can
mount the private gateway socket and its own mTLS identity with no published ports;
it must use the same runtime UID as the socket owner. This CLI does not mount or
access the Docker socket, and it does not perform infrastructure writes.

The shared generated wire API lives in `proto/v1`. Its `GetStackNetwork` and `GetStackTraffic` methods are
reserved for the host's private delegated-user broker; external services cannot
obtain its internal lease from this SDK. Its native UI storage facade remains
administrator-scoped; service storage uses the independent machine grants above.
Managed executable startup and production certificate issuance/rotation remain
separate platform work.

## Develop and verify

From this standalone SDK project root:

```sh
GOWORK=off go test -race ./...
GOWORK=off go vet ./...

# Host integration remains in the Vessel HQ checkout:
# go -C /path/to/Vessel-HQ/backend-src test -race ./api/nativeextensions/... ./api/http/handler/nativeextensions
```

The SDK tests establish real TCP and Unix-domain mTLS connections and cover concurrent reuse,
current registration, permission errors/revocation, default and caller deadlines,
cancellation, invalid/untrusted identities, server-name checks, oversized
responses and mismatched scope. The backend integration suite checks actual grant
enforcement using the same public contract.

Regenerate from the schema; never edit generated Go files directly. Install
`protoc` and these pinned generators, put their binaries on PATH, then run:

```sh
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
./generate.sh
./generate.sh --check
```

The `--check` mode generates into a temporary directory and compares the result,
ignoring only the informational protoc version line. The schema keeps the same
`vessel.extensions.v1` wire service and field numbers used by the original host.

Package this workspace with `python3 scripts/package-source.py --out /tmp/vessel-extension-sdk.tar.gz`. It first builds the independent Go module and includes only the public SDK, schema, types, simple plugin sources, documentation and tooling. It excludes credentials, local IDE state and build output. Package simple plugins using the commands in the root README.
