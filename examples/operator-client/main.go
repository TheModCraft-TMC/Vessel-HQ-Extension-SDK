// vessel-extension-example demonstrates dedicated, scoped extension gRPC access.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	extension "github.com/TheModCraft-TMC/Vessel-HQ/sdk/go"
	"google.golang.org/protobuf/encoding/protojson"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(arguments []string, output io.Writer) error {
	flags := flag.NewFlagSet("vessel-extension-example", flag.ContinueOnError)
	flags.SetOutput(output)
	address := flags.String("address", "127.0.0.1:19444", "Dedicated Vessel extension gRPC listener")
	ca := flags.String("ca", "", "Dedicated CA certificate PEM")
	cert := flags.String("cert", "", "Extension client certificate PEM")
	key := flags.String("key", "", "Extension client private key PEM")
	environmentID := flags.Int64("environment-id", 0, "Approved environment ID to read")
	createCertificates := flags.Bool("create-dev-certs", false, "Create a new local-only development CA and mTLS identities")
	extensionID := flags.String("extension-id", "environment-summary", "Development certificate extension ID")
	packageDigest := flags.String("package-digest", "", "Exact registered sha256 package digest for the development identity")
	out := flags.String("out", "", "New certificate directory; defaults to a new private /tmp directory")
	if err := flags.Parse(arguments); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if *createCertificates {
		paths, err := createDevelopmentCertificates(*out, *extensionID, *packageDigest)
		if err != nil {
			return err
		}
		return json.NewEncoder(output).Encode(paths)
	}
	if *address == "" || *ca == "" || *cert == "" || *key == "" || *environmentID <= 0 {
		return errors.New("address, ca, cert, key, and a positive environment-id are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := extension.Dial(ctx, extension.Config{Address: *address, CAFile: *ca, CertificateFile: *cert, KeyFile: *key})
	if err != nil {
		return err
	}
	defer client.Close()
	registration, err := client.GetRegistration(ctx)
	if err != nil {
		return fmt.Errorf("registration: %w", err)
	}
	environment, err := client.GetEnvironment(ctx, *environmentID)
	if err != nil {
		return fmt.Errorf("environment metadata: %w", err)
	}
	registrationJSON, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(registration)
	if err != nil {
		return err
	}
	environmentJSON, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(environment)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(map[string]json.RawMessage{"registration": registrationJSON, "environment": environmentJSON})
}
