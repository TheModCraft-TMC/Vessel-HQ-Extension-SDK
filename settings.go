package extension

import (
	"context"
	"fmt"
	"unicode/utf8"

	v1 "github.com/TheModCraft-TMC/Vessel-HQ-Extension-SDK/proto/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Settings contains administrator configuration for this package. Secrets are
// sensitive in-memory data; never log, serialize into diagnostics or cache them.
type Settings struct {
	Version int64
	Values  map[string]string
	Secrets map[string]string
}

// GetSettings requires settings.service.read and the current approval revision.
// includeSecrets additionally requires settings.service.secrets.read. The host
// checks both grants afresh; clients should refresh configuration before use.
func (c *Client) GetSettings(ctx context.Context, revision int64, includeSecrets bool) (Settings, error) {
	if revision <= 0 {
		return Settings{}, status.Error(codes.InvalidArgument, "positive approval revision required")
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.gateway.GetSettings(ctx, &v1.GetSettingsRequest{Revision: revision, IncludeSecrets: includeSecrets})
	if err != nil {
		return Settings{}, fmt.Errorf("get extension settings: %w", err)
	}
	if response == nil || response.Version < 0 || (!includeSecrets && len(response.Secrets) != 0) || len(response.Values)+len(response.Secrets) > 64 {
		return Settings{}, status.Error(codes.DataLoss, "invalid extension settings response")
	}
	size := 0
	for _, entries := range []map[string]string{response.Values, response.Secrets} {
		for key, value := range entries {
			if len(key) == 0 || len(key) > 64 || !utf8.ValidString(key) || len(value) > 16384 || !utf8.ValidString(value) {
				return Settings{}, status.Error(codes.DataLoss, "invalid extension settings response")
			}
			size += len(key) + len(value)
		}
	}
	if size > 65536 {
		return Settings{}, status.Error(codes.DataLoss, "invalid extension settings response")
	}
	for key := range response.Secrets {
		if _, exists := response.Values[key]; exists {
			return Settings{}, status.Error(codes.DataLoss, "invalid extension settings response")
		}
	}
	return Settings{Version: response.Version, Values: response.Values, Secrets: response.Secrets}, nil
}
