// Package extension provides the Vessel HQ extension service SDK.
//
// Dial establishes a mutually authenticated gRPC connection and verifies that
// Vessel's registration matches the package identity in the client certificate.
// Reuse the returned Client across goroutines and close it on shutdown. Every RPC
// has a bounded deadline and retains gRPC status codes for callers to inspect.
// Grants are enforced live by Vessel, never cached as authority by this SDK.
package extension
