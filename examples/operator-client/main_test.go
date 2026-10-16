package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	extensionv1 "github.com/TheModCraft-TMC/Vessel-HQ/sdk/go/proto/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

const exampleDigest = "sha256:7352da43730269f8c7720f389d89865efd42364932e0d85f541fb7d9d98af4b9"

func developmentIdentity(t *testing.T) certificatePaths {
	t.Helper()
	paths, err := createDevelopmentCertificates(filepath.Join(t.TempDir(), "identity"), "environment-summary", exampleDigest)
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

func TestDevelopmentCertificatesAreScopedPrivateAndShortLived(t *testing.T) {
	paths := developmentIdentity(t)
	for _, name := range []string{"ca.crt", "ca.key", "server.crt", "server.key", "client.crt", "client.key"} {
		stat, err := os.Stat(filepath.Join(paths.Directory, name))
		if err != nil || stat.Mode().Perm() != 0o600 {
			t.Fatalf("expected private %s: %v", name, err)
		}
	}
	stat, _ := os.Stat(paths.Directory)
	if stat.Mode().Perm() != 0o700 {
		t.Fatal("certificate directory is not private")
	}
	clientPEM, _ := os.ReadFile(paths.Client)
	block, _ := pem.Decode(clientPEM)
	client, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(client.URIs) != 1 || client.URIs[0].String() != paths.Identity || !strings.Contains(paths.Identity, strings.TrimPrefix(exampleDigest, "sha256:")) {
		t.Fatal("client identity is not bound to the exact package")
	}
	if remaining := time.Until(client.NotAfter); remaining <= 23*time.Hour || remaining > 24*time.Hour {
		t.Fatal("development identity must expire within 24 hours")
	}
	serverPEM, _ := os.ReadFile(paths.Server)
	block, _ = pem.Decode(serverPEM)
	server, _ := x509.ParseCertificate(block.Bytes)
	for _, host := range []string{"localhost", "127.0.0.1", "::1"} {
		if err := server.VerifyHostname(host); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDevelopmentCertificatesRefuseExistingDirectory(t *testing.T) {
	directory := t.TempDir()
	sentinel := filepath.Join(directory, "keep")
	_ = os.WriteFile(sentinel, []byte("preserved"), 0o600)
	if _, err := createDevelopmentCertificates(directory, "environment-summary", exampleDigest); err == nil {
		t.Fatal("existing output directory accepted")
	}
	if content, _ := os.ReadFile(sentinel); string(content) != "preserved" {
		t.Fatal("existing data was modified")
	}
}

type exampleGateway struct {
	extensionv1.UnimplementedExtensionGatewayServer
	t        *testing.T
	identity string
}

func (s *exampleGateway) GetRegistration(ctx context.Context, _ *extensionv1.GetRegistrationRequest) (*extensionv1.GetRegistrationResponse, error) {
	remote, ok := peer.FromContext(ctx)
	if !ok {
		s.t.Error("missing authenticated peer")
	} else if info, ok := remote.AuthInfo.(credentials.TLSInfo); !ok || len(info.State.PeerCertificates) != 1 || info.State.PeerCertificates[0].URIs[0].String() != s.identity {
		s.t.Error("incorrect client mTLS identity")
	}
	return &extensionv1.GetRegistrationResponse{ExtensionId: "environment-summary", PackageDigest: exampleDigest, Revision: 1, Status: "approved", RuntimeAvailable: true}, nil
}

func (s *exampleGateway) GetEnvironment(_ context.Context, request *extensionv1.GetEnvironmentRequest) (*extensionv1.GetEnvironmentResponse, error) {
	return &extensionv1.GetEnvironmentResponse{Id: request.EnvironmentId, Name: "Development environment", Type: 1, Status: 1}, nil
}

func TestClientReadsMetadataUsingRealMutualTLS(t *testing.T) {
	paths := developmentIdentity(t)
	certificate, _ := tls.LoadX509KeyPair(paths.Server, paths.ServerKey)
	ca, _ := os.ReadFile(paths.CA)
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(ca)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate}, ClientCAs: pool, ClientAuth: tls.RequireAndVerifyClientCert})))
	extensionv1.RegisterExtensionGatewayServer(server, &exampleGateway{t: t, identity: paths.Identity})
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	go func() { _ = server.Serve(listener) }()
	var output bytes.Buffer
	if err := run([]string{"--address", listener.Addr().String(), "--ca", paths.CA, "--cert", paths.Client, "--key", paths.ClientKey, "--environment-id", "7"}, &output); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Environment struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"environment"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil || result.Environment.ID != "7" || result.Environment.Name != "Development environment" {
		t.Fatalf("incorrect metadata JSON: %v %s", err, output.String())
	}
	if strings.Contains(output.String(), "PRIVATE KEY") {
		t.Fatal("private key exposed in output")
	}
}

func TestClientRejectsMissingConfiguration(t *testing.T) {
	if err := run([]string{"--environment-id", "7"}, &bytes.Buffer{}); err == nil {
		t.Fatal("incomplete mTLS configuration accepted")
	}
}

func TestHelpExitsSuccessfullyWithoutCredentials(t *testing.T) {
	var output bytes.Buffer
	if err := run([]string{"--help"}, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Dedicated Vessel extension gRPC listener") {
		t.Fatal("help must describe the gateway address")
	}
}
