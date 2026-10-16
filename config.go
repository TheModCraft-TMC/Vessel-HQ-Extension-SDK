package extension

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config selects the dedicated extension listener and an operator-issued mTLS
// identity. Certificates are loaded by Dial; private key bytes are never logged.
type Config struct {
	// Address is host:port or unix:///absolute/clean/socket-path. Unix transport
	// requires ServerName and keeps the same mTLS and package-identity checks.
	Address         string
	CAFile          string
	CertificateFile string
	KeyFile         string
	// ServerName verifies a DNS SAN when Address is an IP or Unix-domain socket.
	// Empty uses the TCP host; Unix requires an explicit name. Never disables verification.
	ServerName string
	// RPCTimeout bounds each call, including the registration check during Dial.
	// Zero uses ten seconds. An earlier caller deadline always takes precedence.
	RPCTimeout time.Duration
}

// ConfigFromEnvironment reads VESSEL_EXTENSION_CLIENT_ADDRESS, _CA, _CERT, _KEY,
// optional _SERVER_NAME and _RPC_TIMEOUT (a Go duration, for example "5s").
// It never reads an admin session or creates a certificate on the caller's behalf.
func ConfigFromEnvironment() (Config, error) {
	c := Config{
		Address:         os.Getenv("VESSEL_EXTENSION_CLIENT_ADDRESS"),
		CAFile:          os.Getenv("VESSEL_EXTENSION_CLIENT_CA"),
		CertificateFile: os.Getenv("VESSEL_EXTENSION_CLIENT_CERT"),
		KeyFile:         os.Getenv("VESSEL_EXTENSION_CLIENT_KEY"),
		ServerName:      os.Getenv("VESSEL_EXTENSION_CLIENT_SERVER_NAME"),
	}
	if value := os.Getenv("VESSEL_EXTENSION_CLIENT_RPC_TIMEOUT"); value != "" {
		var err error
		c.RPCTimeout, err = time.ParseDuration(value)
		if err != nil || c.RPCTimeout <= 0 {
			return Config{}, errors.New("VESSEL_EXTENSION_CLIENT_RPC_TIMEOUT must be a positive Go duration")
		}
	}
	return c.normalized()
}

func (c Config) normalized() (Config, error) {
	if strings.HasPrefix(c.Address, "unix://") {
		path := strings.TrimPrefix(c.Address, "unix://")
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" || strings.ContainsAny(path, "?#%@\\ \t\r\n\x00") {
			return Config{}, errors.New("extension Unix address must be unix:///absolute/clean/socket-path")
		}
		if strings.TrimSpace(c.ServerName) == "" {
			return Config{}, errors.New("extension Unix transport requires a TLS server name")
		}
	} else {
		host, port, err := net.SplitHostPort(c.Address)
		n, portErr := strconv.Atoi(port)
		if err != nil || host == "" || strings.ContainsAny(host, "/?#@\\ \t\r\n") || portErr != nil || n < 1 || n > 65535 {
			return Config{}, errors.New("extension address must be host:port or unix:///absolute/socket-path")
		}
	}
	for _, item := range []struct{ name, value string }{{"CA", c.CAFile}, {"certificate", c.CertificateFile}, {"private key", c.KeyFile}} {
		if strings.TrimSpace(item.value) == "" {
			return Config{}, fmt.Errorf("extension client %s file is required", item.name)
		}
	}
	if c.RPCTimeout < 0 {
		return Config{}, errors.New("extension RPC timeout must be positive")
	}
	if c.RPCTimeout == 0 {
		c.RPCTimeout = 10 * time.Second
	}
	return c, nil
}
