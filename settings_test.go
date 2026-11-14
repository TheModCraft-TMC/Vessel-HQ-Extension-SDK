package extension_test

import (
	"context"
	"testing"

	extension "github.com/TheModCraft-TMC/Vessel-HQ-Extension-SDK"
	v1 "github.com/TheModCraft-TMC/Vessel-HQ-Extension-SDK/proto/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (g *gateway) GetSettings(ctx context.Context, r *v1.GetSettingsRequest) (*v1.ExtensionSettings, error) {
	if g.denied.Load() {
		return nil, status.Error(codes.PermissionDenied, "revoked")
	}
	if r.Revision != 1 {
		return nil, status.Error(codes.FailedPrecondition, "revision changed")
	}
	result := &v1.ExtensionSettings{Version: 3, Values: map[string]string{"enabled": "true"}}
	if r.IncludeSecrets {
		result.Secrets = map[string]string{"token": "private-test-value"}
	}
	return result, nil
}

func TestSettingsOverMutualTLS(t *testing.T) {
	for _, unix := range []bool{false, true} {
		t.Run(map[bool]string{false: "tcp", true: "unix"}[unix], func(t *testing.T) {
			f := setupTransport(t, nil, nil, unix)
			c, err := extension.Dial(context.Background(), f.config)
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			settings, err := c.GetSettings(context.Background(), 1, false)
			if err != nil || settings.Version != 3 || settings.Values["enabled"] != "true" || len(settings.Secrets) != 0 {
				t.Fatalf("public settings contract failed: %v", err)
			}
			settings, err = c.GetSettings(context.Background(), 1, true)
			if err != nil || settings.Secrets["token"] != "private-test-value" {
				t.Fatalf("secret settings contract failed: %v", err)
			}
			if _, err := c.GetSettings(context.Background(), 0, false); status.Code(err) != codes.InvalidArgument {
				t.Fatal("invalid revision accepted")
			}
			if _, err := c.GetSettings(context.Background(), 2, false); status.Code(err) != codes.FailedPrecondition {
				t.Fatal("stale revision accepted")
			}
			f.gateway.denied.Store(true)
			if _, err := c.GetSettings(context.Background(), 1, true); status.Code(err) != codes.PermissionDenied {
				t.Fatal("revocation not respected")
			}
		})
	}
}
