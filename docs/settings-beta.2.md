# Settings RPC release preparation

Target version: `v0.1.0-beta.2`. This checkout is a release candidate, not a published module.

`Client.GetSettings(ctx, revision, includeSecrets)` reads configuration for the certificate-bound package. It requires `settings.service.read`; requesting secrets additionally requires `settings.service.secrets.read`. Approval revision and both grants are checked on every call by the host. No environment grant or UI trust substitutes for these machine grants. No machine writes are provided.

Values remain strings. Treat returned secrets as sensitive in-memory data: do not log them, include them in diagnostics or persist them to extension notes. Old hosts return gRPC Unimplemented. This additive RPC preserves existing service and protobuf field numbers.

The maintainer selected `github.com/TheModCraft-TMC/Vessel-HQ-Extension-SDK` as the canonical independent module path. Publish the root tag `v0.1.0-beta.2` from this repository, then update Vessel's checksum using the published module and verify without a workspace override. The historical beta.1 module and tags remain immutable. Local race tests, vet and protobuf generation checks passed using Go 1.26.10. Publication has not occurred.
