# Vessel HQ extension SDK

This workspace owns the independent Go gRPC SDK, shared protobuf contract, native UI types, and simple extension demos. The Stack Network Overview React Flow extension remains in Vessel HQ.

Preserve the wire service, field numbers, package-bound mTLS checks, least privilege grants and note compare-and-swap behavior. Never add Docker sockets, credentials, runtime certificates or generated build artifacts. Read README.md before edits. Regenerate protobuf Go files with ./generate.sh; do not hand-edit generated output.

Run GOWORK=off go test -race ./... and GOWORK=off go vet ./... for Go changes. Run ./generate.sh --check for schema changes. Check simple plugins with the Vessel trusted builder; upload never authorizes executable startup. Do not change the published module identity until a new canonical Git remote and module release are authorized.
