// environment-summary is an independent extension executable using only the SDK.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	extension "github.com/TheModCraft-TMC/Vessel-HQ/sdk/go"
)

func main() {
	id := flag.Int64("environment-id", 1, "Approved environment ID")
	flag.Parse()
	if err := run(*id); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(id int64) error {
	config, err := extension.ConfigFromEnvironment()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client, err := extension.Dial(ctx, config)
	if err != nil {
		return err
	}
	defer client.Close()
	environment, err := client.GetEnvironment(ctx, id)
	if err != nil {
		return err
	}
	// Present an ordinary JSON DTO rather than protobuf implementation fields.
	return json.NewEncoder(os.Stdout).Encode(struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		Type   int32  `json:"type"`
		Status int32  `json:"status"`
	}{environment.GetId(), environment.GetName(), environment.GetType(), environment.GetStatus()})
}
