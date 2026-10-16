# Read environment metadata through extension gRPC

This development client calls the real `GetRegistration` and `GetEnvironment`
protobuf RPCs through the [independent Go SDK](../../docs/go-sdk.md) over
mutually authenticated TLS 1.3. Connection setup, certificate validation, bounded
calls and gRPC error handling live in the SDK. It prints registration details
and environment ID/name/type/status as JSON; it receives no credentials, endpoint
addresses or engine sockets. Protobuf 64-bit IDs appear as JSON strings.

Run these commands from the repository root. Build the example using the Go
version declared in `go.mod`:

```sh
go build -o /tmp/vessel-extension-example ./examples/operator-client
/tmp/vessel-extension-example --create-dev-certs \
  --extension-id environment-summary \
  --package-digest sha256:7352da43730269f8c7720f389d89865efd42364932e0d85f541fb7d9d98af4b9 \
  --out /tmp/vessel-extension-dev-mtls
```

The output directory must be new. Omit `--out` for a fresh private `/tmp` directory.
The command prints certificate paths and the digest-bound SPIFFE identity, never
private key contents. It creates a dedicated 24-hour development CA, a localhost
server certificate and an extension client certificate. Directory permissions are
0700 and certificate/key files are 0600. No system trust store is changed.

Configure the disposable local Vessel backend with these four variables before
starting it through the normal development workflow:

```sh
export VESSEL_EXTENSION_GRPC_ADDRESS=127.0.0.1:19444
export VESSEL_EXTENSION_GRPC_CERT=/tmp/vessel-extension-dev-mtls/server.crt
export VESSEL_EXTENSION_GRPC_KEY=/tmp/vessel-extension-dev-mtls/server.key
export VESSEL_EXTENSION_GRPC_CLIENT_CA=/tmp/vessel-extension-dev-mtls/ca.crt
```

In Vessel's administrator extension screen, register the reference manifest from
`plugins/environment-summary/manifest.json`. Add a requested
`environments.read` permission with the existing development environment's numeric
ID, for example `{"capability":"environments.read","environmentIds":[1]}`.
Approve that scoped grant and explicitly enable background service reads
(`serviceReadEnabled: true` in the approval request). The native UI trust grant is
separate; a service-only extension does not require it. Registration approval alone
does not enable background reads.

Keep the certificate identity's extension ID and package digest exactly equal to
the approved manifest. A new package digest requires a new client identity and
approval. Current grants and service-read approval are checked on every RPC.

```sh
/tmp/vessel-extension-example \
  --address 127.0.0.1:19444 \
  --ca /tmp/vessel-extension-dev-mtls/ca.crt \
  --cert /tmp/vessel-extension-dev-mtls/client.crt \
  --key /tmp/vessel-extension-dev-mtls/client.key \
  --environment-id 1
```

Successful output contains the actual stored environment metadata. Reading an
unapproved environment or disabling service reads returns a gRPC permission error.
The client never offers plaintext or a certificate-verification bypass and bounds
all calls, including connection registration verification, with a shared ten-second deadline. Keep the dedicated listener local
for this development setup. These certificates are development fixtures, not a
production identity provisioning or rotation service.

```sh
go test ./examples/operator-client
```

Tests perform an actual local TCP/mTLS handshake with a test gRPC service and check
metadata JSON, exact client identity, private file permissions, localhost SANs,
short certificate lifetime and refusal to overwrite existing certificate output.
