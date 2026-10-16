package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type certificatePaths struct {
	Directory string `json:"directory"`
	CA        string `json:"ca"`
	Server    string `json:"serverCert"`
	ServerKey string `json:"serverKey"`
	Client    string `json:"clientCert"`
	ClientKey string `json:"clientKey"`
	Identity  string `json:"identity"`
}

// This creates an isolated, short-lived development CA. It never modifies the
// machine trust store or issues identities using a production CA.
func createDevelopmentCertificates(directory, extensionID, digest string) (result certificatePaths, err error) {
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,126}[a-z0-9]$`).MatchString(extensionID) || strings.Contains(extensionID, "..") || !regexp.MustCompile(`^sha256:[a-f0-9]{64}$`).MatchString(digest) {
		return result, errors.New("valid extension-id and exact sha256 package-digest are required")
	}
	if directory == "" {
		directory, err = os.MkdirTemp("/tmp", "vessel-extension-grpc-")
	} else {
		directory, err = filepath.Abs(directory)
		if err == nil {
			err = os.Mkdir(directory, 0o700)
		}
	}
	if err != nil {
		return result, fmt.Errorf("create new private development certificate directory: %w", err)
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(directory)
		}
	}()
	now := time.Now().UTC()
	newTemplate := func(name string) (*x509.Certificate, error) {
		serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
		if err != nil {
			return nil, err
		}
		return &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: name}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, BasicConstraintsValid: true}, nil
	}
	caTemplate, err := newTemplate("Vessel local extension development CA")
	if err != nil {
		return result, err
	}
	caTemplate.IsCA = true
	caTemplate.MaxPathLenZero = true
	caTemplate.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageCRLSign
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return result, err
	}
	writeIdentity := func(prefix string, template, parent *x509.Certificate, signer *ecdsa.PrivateKey, privateKey *ecdsa.PrivateKey) error {
		certificate, err := x509.CreateCertificate(rand.Reader, template, parent, &privateKey.PublicKey, signer)
		if err != nil {
			return err
		}
		key, err := x509.MarshalPKCS8PrivateKey(privateKey)
		if err != nil {
			return err
		}
		for name, bytes := range map[string][]byte{prefix + ".crt": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate}), prefix + ".key": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})} {
			file, err := os.OpenFile(filepath.Join(directory, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
			if err != nil {
				return err
			}
			_, writeErr := file.Write(bytes)
			closeErr := file.Close()
			if writeErr != nil {
				return writeErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
		return nil
	}
	if err = writeIdentity("ca", caTemplate, caTemplate, caKey, caKey); err != nil {
		return result, err
	}
	uri, err := url.Parse("spiffe://vessel.local/extensions/" + extensionID + "/packages/" + strings.TrimPrefix(digest, "sha256:"))
	if err != nil {
		return result, err
	}
	for _, name := range []string{"server", "client"} {
		template, templateErr := newTemplate("Vessel local extension " + name)
		if templateErr != nil {
			return result, templateErr
		}
		if name == "server" {
			template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
			template.DNSNames = []string{"localhost"}
			template.IPAddresses = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
		} else {
			template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
			template.URIs = []*url.URL{uri}
		}
		key, keyErr := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if keyErr != nil {
			return result, keyErr
		}
		if err = writeIdentity(name, template, caTemplate, caKey, key); err != nil {
			return result, err
		}
	}
	return certificatePaths{Directory: directory, CA: filepath.Join(directory, "ca.crt"), Server: filepath.Join(directory, "server.crt"), ServerKey: filepath.Join(directory, "server.key"), Client: filepath.Join(directory, "client.crt"), ClientKey: filepath.Join(directory, "client.key"), Identity: uri.String()}, nil
}
