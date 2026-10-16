package extension_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	extension "github.com/TheModCraft-TMC/Vessel-HQ/sdk/go"
	v1 "github.com/TheModCraft-TMC/Vessel-HQ/sdk/go/proto/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

const testID = "org.example.sdk"

var testDigest = "sha256:" + strings.Repeat("a", 64)

type gateway struct {
	v1.UnimplementedExtensionGatewayServer
	t             *testing.T
	registrations atomic.Int64
	denied        atomic.Bool
	registration  func() *v1.GetRegistrationResponse
	read          func(context.Context, int64) (*v1.GetEnvironmentResponse, error)
}

func (g *gateway) GetRegistration(ctx context.Context, _ *v1.GetRegistrationRequest) (*v1.GetRegistrationResponse, error) {
	g.registrations.Add(1)
	p, ok := peer.FromContext(ctx)
	if !ok {
		g.t.Error("missing authenticated peer")
	} else if info, ok := p.AuthInfo.(credentials.TLSInfo); !ok || info.State.Version != tls.VersionTLS13 || len(info.State.VerifiedChains) == 0 {
		g.t.Error("expected verified TLS 1.3 client identity")
	}
	if g.denied.Load() {
		return nil, status.Error(codes.PermissionDenied, "revoked")
	}
	if g.registration != nil {
		return g.registration(), nil
	}
	return &v1.GetRegistrationResponse{ExtensionId: testID, PackageDigest: testDigest, Revision: g.registrations.Load(), Status: "approved", RuntimeAvailable: true}, nil
}
func (g *gateway) GetEnvironment(ctx context.Context, r *v1.GetEnvironmentRequest) (*v1.GetEnvironmentResponse, error) {
	if g.denied.Load() || r.EnvironmentId != 7 {
		return nil, status.Error(codes.PermissionDenied, "outside approved scope")
	}
	if g.read != nil {
		return g.read(ctx, r.EnvironmentId)
	}
	return &v1.GetEnvironmentResponse{Id: r.EnvironmentId, Name: "SDK environment", Type: 1, Status: 1}, nil
}

type fixture struct {
	config  extension.Config
	gateway *gateway
}

func setup(t *testing.T, modify func(*x509.Certificate), g *gateway) fixture {
	t.Helper()
	return setupTransport(t, modify, g, false)
}

func setupTransport(t *testing.T, modify func(*x509.Certificate), g *gateway, unix bool) fixture {
	t.Helper()
	root := t.TempDir()
	now := time.Now()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "SDK test CA"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	ca, err = x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	write := func(name string, data []byte) string {
		t.Helper()
		p := filepath.Join(root, name)
		if err := os.WriteFile(p, data, 0600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	caPath := write("ca.crt", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	issue := func(name string, serial int64, server bool) (tls.Certificate, string, string) {
		t.Helper()
		k, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if e != nil {
			t.Fatal(e)
		}
		template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
		if server {
			template.DNSNames = []string{"sdk.test"}
			template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		} else {
			u, _ := url.Parse("spiffe://vessel.local/extensions/" + testID + "/packages/" + strings.TrimPrefix(testDigest, "sha256:"))
			template.URIs = []*url.URL{u}
			if modify != nil {
				modify(template)
			}
		}
		cert, e := x509.CreateCertificate(rand.Reader, template, ca, &k.PublicKey, key)
		if e != nil {
			t.Fatal(e)
		}
		private, e := x509.MarshalPKCS8PrivateKey(k)
		if e != nil {
			t.Fatal(e)
		}
		cp := write(name+".crt", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert}))
		kp := write(name+".key", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private}))
		pair, e := tls.LoadX509KeyPair(cp, kp)
		if e != nil {
			t.Fatal(e)
		}
		return pair, cp, kp
	}
	serverCert, _, _ := issue("server", 2, true)
	_, clientCert, clientKey := issue("client", 3, false)
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	network, address := "tcp", "127.0.0.1:0"
	if unix {
		dir, e := os.MkdirTemp("/tmp", "v-sdk-")
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { _ = os.RemoveAll(dir) })
		network, address = "unix", filepath.Join(dir, "gateway.sock")
	}
	listener, err := net.Listen(network, address)
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{serverCert}, ClientCAs: pool, ClientAuth: tls.RequireAndVerifyClientCert})))
	if g == nil {
		g = &gateway{}
	}
	g.t = t
	v1.RegisterExtensionGatewayServer(server, g)
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	go func() { _ = server.Serve(listener) }()
	clientAddress := listener.Addr().String()
	if unix {
		clientAddress = "unix://" + clientAddress
	}
	return fixture{extension.Config{Address: clientAddress, CAFile: caPath, CertificateFile: clientCert, KeyFile: clientKey, ServerName: "sdk.test", RPCTimeout: time.Second}, g}
}

func dial(t *testing.T, f fixture) *extension.Client {
	t.Helper()
	c, err := extension.Dial(context.Background(), f.config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}
func requireCode(t *testing.T, err error, code codes.Code) {
	t.Helper()
	if status.Code(err) != code {
		t.Fatalf("expected %s, got %v", code, err)
	}
}

func TestMutualTLSReuseConcurrentCallsAndLiveRevocation(t *testing.T) {
	f := setup(t, nil, nil)
	c := dial(t, f)
	r, err := c.GetRegistration(context.Background())
	if err != nil || r.GetRevision() != 2 {
		t.Fatalf("registration was not refreshed: %v %v", r, err)
	}
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			env, err := c.GetEnvironment(context.Background(), 7)
			if err != nil || env.GetName() != "SDK environment" {
				t.Errorf("read: %v %v", env, err)
			}
		})
	}
	wg.Wait()
	_, err = c.GetEnvironment(context.Background(), 8)
	requireCode(t, err, codes.PermissionDenied)
	_, err = c.GetEnvironment(context.Background(), 0)
	requireCode(t, err, codes.InvalidArgument)
	f.gateway.denied.Store(true)
	_, err = c.GetEnvironment(context.Background(), 7)
	requireCode(t, err, codes.PermissionDenied)
	_, err = c.GetRegistration(context.Background())
	requireCode(t, err, codes.PermissionDenied)
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = c.GetEnvironment(context.Background(), 7)
	requireCode(t, err, codes.Canceled)
}

func TestCallTimeoutAndCallerCancellation(t *testing.T) {
	g := &gateway{read: func(ctx context.Context, _ int64) (*v1.GetEnvironmentResponse, error) {
		<-ctx.Done()
		return nil, status.FromContextError(ctx.Err()).Err()
	}}
	f := setup(t, nil, g)
	f.config.RPCTimeout = 250 * time.Millisecond
	c := dial(t, f)
	start := time.Now()
	_, err := c.GetEnvironment(context.Background(), 7)
	requireCode(t, err, codes.DeadlineExceeded)
	if time.Since(start) > 2*time.Second {
		t.Fatal("SDK default call deadline was not applied")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start = time.Now()
	_, err = c.GetEnvironment(ctx, 7)
	requireCode(t, err, codes.DeadlineExceeded)
	if time.Since(start) > 200*time.Millisecond {
		t.Fatal("earlier caller deadline did not take precedence")
	}
	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	_, err = c.GetEnvironment(ctx, 7)
	requireCode(t, err, codes.Canceled)
}

func TestResponseLimitsAndScopeIdentity(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response *v1.GetEnvironmentResponse
		code     codes.Code
	}{
		{"wrong scope", &v1.GetEnvironmentResponse{Id: 8}, codes.DataLoss},
		{"oversized", &v1.GetEnvironmentResponse{Id: 7, Name: strings.Repeat("x", 300*1024)}, codes.ResourceExhausted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t, nil, &gateway{read: func(context.Context, int64) (*v1.GetEnvironmentResponse, error) { return tc.response, nil }})
			c := dial(t, f)
			_, err := c.GetEnvironment(context.Background(), 7)
			requireCode(t, err, tc.code)
		})
	}
}

func TestDialRejectsRegistrationIdentityMismatch(t *testing.T) {
	f := setup(t, nil, &gateway{registration: func() *v1.GetRegistrationResponse {
		return &v1.GetRegistrationResponse{ExtensionId: "other.extension", PackageDigest: testDigest, Revision: 1, Status: "approved"}
	}})
	c, err := extension.Dial(context.Background(), f.config)
	requireCode(t, err, codes.DataLoss)
	if c != nil {
		t.Fatal("failed Dial leaked a usable client")
	}
}

func TestCertificateValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		modify func(*x509.Certificate)
		want   string
	}{
		{"no URI", func(c *x509.Certificate) { c.URIs = nil }, "one package-bound URI"},
		{"multiple URIs", func(c *x509.Certificate) { c.URIs = append(c.URIs, c.URIs[0]) }, "one package-bound URI"},
		{"foreign authority", func(c *x509.Certificate) { c.URIs[0].Host = "other.example" }, "invalid package identity"},
		{"query", func(c *x509.Certificate) { c.URIs[0].RawQuery = "authority=admin" }, "invalid package identity"},
		{"expired", func(c *x509.Certificate) { c.NotAfter = time.Now().Add(-time.Second) }, "not currently valid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t, tc.modify, nil)
			c, err := extension.Dial(context.Background(), f.config)
			if err == nil || !strings.Contains(err.Error(), tc.want) || c != nil {
				t.Fatalf("expected %q: %v", tc.want, err)
			}
		})
	}
}

func TestTLSRejectsWrongServerAndUntrustedCA(t *testing.T) {
	f := setup(t, nil, nil)
	t.Run("wrong server", func(t *testing.T) {
		c := f.config
		c.ServerName = "wrong.test"
		_, err := extension.Dial(context.Background(), c)
		requireCode(t, err, codes.Unavailable)
	})
	t.Run("foreign CA", func(t *testing.T) {
		foreign := setup(t, nil, nil)
		c := f.config
		c.CAFile = foreign.config.CAFile
		_, err := extension.Dial(context.Background(), c)
		requireCode(t, err, codes.Unavailable)
	})
	t.Run("malformed CA", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "ca")
		if err := os.WriteFile(path, []byte("not a certificate"), 0600); err != nil {
			t.Fatal(err)
		}
		c := f.config
		c.CAFile = path
		_, err := extension.Dial(context.Background(), c)
		if err == nil || !strings.Contains(err.Error(), "invalid extension CA") {
			t.Fatal(err)
		}
	})
	t.Run("missing key", func(t *testing.T) {
		c := f.config
		c.KeyFile = filepath.Join(t.TempDir(), "missing")
		_, err := extension.Dial(context.Background(), c)
		if err == nil {
			t.Fatal("missing key accepted")
		}
	})
}

func TestEnvironmentConfiguration(t *testing.T) {
	for k, v := range map[string]string{"ADDRESS": "localhost:19444", "CA": "ca.crt", "CERT": "client.crt", "KEY": "client.key", "SERVER_NAME": "sdk.test", "RPC_TIMEOUT": "3s"} {
		t.Setenv("VESSEL_EXTENSION_CLIENT_"+k, v)
	}
	c, err := extension.ConfigFromEnvironment()
	if err != nil || c.RPCTimeout != 3*time.Second || c.ServerName != "sdk.test" {
		t.Fatalf("config: %+v %v", c, err)
	}
	for _, value := range []string{"", "http://localhost:19444", "dns:///localhost:19444", "localhost:0", "localhost:65536", "localhost:abc", ":19444"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("VESSEL_EXTENSION_CLIENT_ADDRESS", value)
			if _, err := extension.ConfigFromEnvironment(); err == nil {
				t.Fatal("invalid address accepted")
			}
		})
	}
	for _, value := range []string{"0s", "-1s", "nonsense"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("VESSEL_EXTENSION_CLIENT_RPC_TIMEOUT", value)
			if _, err := extension.ConfigFromEnvironment(); err == nil {
				t.Fatal("invalid timeout accepted")
			}
		})
	}
	t.Setenv("VESSEL_EXTENSION_CLIENT_RPC_TIMEOUT", "")
	c, err = extension.ConfigFromEnvironment()
	if err != nil || c.RPCTimeout != 10*time.Second {
		t.Fatalf("default timeout: %+v %v", c, err)
	}
	t.Setenv("VESSEL_EXTENSION_CLIENT_CERT", "")
	if _, err := extension.ConfigFromEnvironment(); err == nil {
		t.Fatal("partial configuration accepted")
	}
}

func TestUnixMutualTLSAndServerVerification(t *testing.T) {
	f := setupTransport(t, nil, nil, true)
	c := dial(t, f)
	response, err := c.GetEnvironment(context.Background(), 7)
	if err != nil || response.GetName() != "SDK environment" {
		t.Fatalf("Unix RPC: %v %v", response, err)
	}
	f.config.ServerName = "wrong.test"
	f.config.RPCTimeout = 100 * time.Millisecond
	if _, err := extension.Dial(context.Background(), f.config); err == nil {
		t.Fatal("Unix transport bypassed TLS server verification")
	}
	f.config.ServerName = ""
	if _, err := extension.Dial(context.Background(), f.config); err == nil {
		t.Fatal("Unix transport accepted missing TLS server name")
	}
}

func TestUnixConfigurationRejectsAmbiguousTargets(t *testing.T) {
	for k, v := range map[string]string{"CA": "ca", "CERT": "cert", "KEY": "key", "SERVER_NAME": "sdk.test"} {
		t.Setenv("VESSEL_EXTENSION_CLIENT_"+k, v)
	}
	for _, address := range []string{"unix://relative.sock", "unix:///", "unix:///tmp/../gateway.sock", "unix:///tmp/a.sock?x=1", "unix:///tmp/a%2fb.sock", "unix://authority/tmp/a.sock", "unix-abstract:gateway", "unix:///tmp/a.sock#fragment"} {
		t.Run(address, func(t *testing.T) {
			t.Setenv("VESSEL_EXTENSION_CLIENT_ADDRESS", address)
			if _, err := extension.ConfigFromEnvironment(); err == nil {
				t.Fatal("ambiguous Unix target accepted")
			}
		})
	}
	t.Setenv("VESSEL_EXTENSION_CLIENT_ADDRESS", "unix:///tmp/gateway.sock")
	if _, err := extension.ConfigFromEnvironment(); err != nil {
		t.Fatal(err)
	}
}
