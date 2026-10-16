# Extraction verification: 2026-10-16

This local workspace was extracted from Vessel HQ feature commit `27ddd4d89fb3bac3b9c7f73f916dab757cd02abc`. The published SDK remains `github.com/TheModCraft-TMC/Vessel-HQ/sdk/go@v0.1.0-beta.1`; this extraction task did not create a remote or publish a module release. It preserved externally created initial commit `901cafd` and `origin` at `https://github.com/TheModCraft-TMC/Vessel-HQ-Extension-SDK`.

Fresh checks in this workspace: Go 1.26.6 `GOWORK=off go test -race ./...`, `go vet ./...`, `go build -mod=readonly ./...`, and pinned `./generate.sh --check` passed. All environment-summary, persistent-notes and operator-client example executables built for Linux amd64 and arm64.

The trusted host builder packaged the extracted persistent-notes UI and environment-summary service with a real ARM64 Linux executable. Persistent-notes source digest remained `sha256:4f8412f4c00a671a88a3ea806aef79572b6c44ff0ccb40ae232c727bd7751ad1`; archive digest remained `sha256:c6b64ced40f868c4a91da3c29b74489322e53d85f24e42e640e47356952bc553`. Its three UI tests passed through the host test harness using a temporary source-identical copy, removed afterward. Native UI is compiled and tested inside a trusted Vessel host, rather than providing a separate web runtime.

Vessel host race integration passed with `GOWORK=off` and `-mod=readonly` against the published SDK. Host tooling passed 67 tests. Frontend architecture guards and typecheck passed; port 8999 browser smoke retained installed network/traffic and notes payloads. Host and standalone workflow syntax passed actionlint with shellcheck disabled.

These results cover extraction and compatibility. They do not establish managed service startup, infrastructure mutations, reverse routing or production certificate lifecycle. Existing beta.1 tags and their release pipeline remain immutable. Local artifacts exclude credentials, runtime certificates, IDE state and datastore files.
