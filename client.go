package extension

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	extensionv1 "github.com/TheModCraft-TMC/Vessel-HQ/sdk/go/proto/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

// Client owns a reusable, concurrency-safe connection to Vessel. Use Dial to
// construct it. Configuration and certificates are immutable for its lifetime;
// create a new client when rotating credentials.
type Client struct {
	connection    *grpc.ClientConn
	gateway       extensionv1.ExtensionGatewayClient
	timeout       time.Duration
	extensionID   string
	packageDigest string
	closeOnce     sync.Once
	closeErr      error
}

// Dial loads and validates mTLS material, then checks registration over the real
// connection before returning. Connection establishment and the check are bounded
// by Config.RPCTimeout and ctx. Any failure closes the new connection.
func Dial(ctx context.Context, config Config) (*Client, error) {
	c, err := config.normalized()
	if err != nil {
		return nil, err
	}
	transport, id, digest, err := clientTransport(c)
	if err != nil {
		return nil, err
	}
	target := "passthrough:///" + c.Address
	if strings.HasPrefix(c.Address, "unix://") {
		target = c.Address
	}
	connection, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(transport),
		grpc.WithDisableServiceConfig(), grpc.WithDisableRetry(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(256*1024), grpc.MaxCallSendMsgSize(64*1024)),
	)
	if err != nil {
		return nil, fmt.Errorf("create extension connection: %w", err)
	}
	client := &Client{connection: connection, gateway: extensionv1.NewExtensionGatewayClient(connection), timeout: c.RPCTimeout, extensionID: id, packageDigest: digest}
	if _, err := client.GetRegistration(ctx); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("connect extension gateway: %w", err)
	}
	return client, nil
}

// Close releases the connection. It may be called more than once.
func (c *Client) Close() error {
	c.closeOnce.Do(func() { c.closeErr = c.connection.Close() })
	return c.closeErr
}

// GetRegistration fetches current package approval and grants. This is a fresh
// diagnostic response, not a promise that a later operation is authorized.
func (c *Client) GetRegistration(ctx context.Context) (*extensionv1.GetRegistrationResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.gateway.GetRegistration(ctx, &extensionv1.GetRegistrationRequest{})
	if err != nil {
		return nil, fmt.Errorf("get extension registration: %w", err)
	}
	if response.GetExtensionId() != c.extensionID || response.GetPackageDigest() != c.packageDigest || response.GetRevision() <= 0 || response.GetStatus() != "approved" {
		return nil, status.Error(codes.DataLoss, "gateway registration does not match the approved client certificate identity")
	}
	return response, nil
}

// GetEnvironment reads approved metadata only (ID, name, type and status).
// It requires the environments.read grant and unattended service-read approval.
// Revoked access is returned immediately as a gRPC PermissionDenied error.
func (c *Client) GetEnvironment(ctx context.Context, environmentID int64) (*extensionv1.GetEnvironmentResponse, error) {
	if environmentID <= 0 {
		return nil, status.Error(codes.InvalidArgument, "positive environment ID required")
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.gateway.GetEnvironment(ctx, &extensionv1.GetEnvironmentRequest{EnvironmentId: environmentID})
	if err != nil {
		return nil, fmt.Errorf("get extension environment: %w", err)
	}
	if response.GetId() != environmentID {
		return nil, status.Error(codes.DataLoss, "gateway returned a different environment")
	}
	return response, nil
}

var identityPattern = regexp.MustCompile(`^/extensions/([a-z0-9][a-z0-9.-]{1,126}[a-z0-9])/packages/([a-f0-9]{64})$`)

func clientTransport(config Config) (credentials.TransportCredentials, string, string, error) {
	pem, err := os.ReadFile(config.CAFile)
	if err != nil {
		return nil, "", "", fmt.Errorf("read extension CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, "", "", errors.New("invalid extension CA certificate")
	}
	certificate, err := tls.LoadX509KeyPair(config.CertificateFile, config.KeyFile)
	if err != nil {
		return nil, "", "", fmt.Errorf("load extension client identity: %w", err)
	}
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		return nil, "", "", fmt.Errorf("parse extension client certificate: %w", err)
	}
	now := time.Now()
	if now.Before(leaf.NotBefore) || !now.Before(leaf.NotAfter) {
		return nil, "", "", errors.New("extension client certificate is not currently valid")
	}
	if len(leaf.URIs) != 1 {
		return nil, "", "", errors.New("extension client certificate requires one package-bound URI SAN")
	}
	uri := leaf.URIs[0]
	parts := identityPattern.FindStringSubmatch(uri.Path)
	if uri.Scheme != "spiffe" || uri.Host != "vessel.local" || uri.User != nil || uri.RawQuery != "" || uri.ForceQuery || uri.Fragment != "" || uri.RawPath != "" || len(parts) != 3 || strings.Contains(parts[1], "..") {
		return nil, "", "", errors.New("extension client certificate has an invalid package identity")
	}
	transport := credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pool, Certificates: []tls.Certificate{certificate}, ServerName: config.ServerName})
	return transport, parts[1], "sha256:" + parts[2], nil
}
