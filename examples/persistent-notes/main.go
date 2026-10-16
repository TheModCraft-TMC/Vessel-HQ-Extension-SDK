// persistent-notes reads or updates extension-owned notes through the SDK.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	extension "github.com/TheModCraft-TMC/Vessel-HQ/sdk/go"
	v1 "github.com/TheModCraft-TMC/Vessel-HQ/sdk/go/proto/v1"
)

type options struct {
	text     string
	update   bool
	showText bool
}
type noteClient interface {
	GetRegistration(context.Context) (*v1.GetRegistrationResponse, error)
	GetNote(context.Context, int64) (extension.Note, error)
	PutNote(context.Context, int64, extension.Note) (extension.Note, error)
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func parseOptions(arguments []string, output io.Writer) (options, error) {
	var value options
	flags := flag.NewFlagSet("persistent-notes", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.StringVar(&value.text, "text", "", "Replace the note text (an explicit empty string clears it)")
	flags.BoolVar(&value.showText, "show-text", false, "Include note text in JSON output; disabled by default")
	if err := flags.Parse(arguments); err != nil {
		return value, err
	}
	if flags.NArg() != 0 {
		return value, errors.New("unexpected positional arguments")
	}
	flags.Visit(func(item *flag.Flag) {
		if item.Name == "text" {
			value.update = true
		}
	})
	return value, nil
}
func run(ctx context.Context, arguments []string, output io.Writer) error {
	value, err := parseOptions(arguments, output)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	config, err := extension.ConfigFromEnvironment()
	if err != nil {
		return err
	}
	client, err := extension.Dial(ctx, config)
	if err != nil {
		return err
	}
	defer client.Close()
	return execute(ctx, value, client, output)
}
func execute(ctx context.Context, value options, client noteClient, output io.Writer) error {
	registration, err := client.GetRegistration(ctx)
	if err != nil {
		return err
	}
	note, err := client.GetNote(ctx, registration.GetRevision())
	if err != nil {
		return err
	}
	if value.update {
		note.Text = value.text
		note, err = client.PutNote(ctx, registration.GetRevision(), note)
		if err != nil {
			return err
		} // Deliberately do not replay writes after a conflict.
	}
	result := struct {
		Version int64   `json:"version"`
		Saved   bool    `json:"saved"`
		Text    *string `json:"text,omitempty"`
	}{Version: note.Version, Saved: value.update}
	if value.showText {
		result.Text = &note.Text
	}
	return json.NewEncoder(output).Encode(result)
}
