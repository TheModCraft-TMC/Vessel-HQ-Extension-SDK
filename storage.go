package extension

import (
	"context"
	"fmt"
	"math"
	"unicode/utf8"

	v1 "github.com/TheModCraft-TMC/Vessel-HQ/sdk/go/proto/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Note is extension-owned text shared with its approved native UI, not a secret
// store or infrastructure mutation. Version is the optimistic concurrency token.
type Note struct {
	Text    string
	Version int64
}

// GetNote requires the explicit storage.service.read grant. revision is obtained
// from GetRegistration; the host rejects a changed approval revision.
func (c *Client) GetNote(ctx context.Context, revision int64) (Note, error) {
	if revision <= 0 {
		return Note{}, status.Error(codes.InvalidArgument, "positive approval revision required")
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.gateway.GetNote(ctx, &v1.GetNoteRequest{Revision: revision})
	if err != nil {
		return Note{}, fmt.Errorf("get extension note: %w", err)
	}
	if response == nil || response.Version < 0 || len(response.Text) > 16384 || !utf8.ValidString(response.Text) {
		return Note{}, status.Error(codes.DataLoss, "invalid extension note response")
	}
	return Note{Text: response.Text, Version: response.Version}, nil
}

// PutNote requires storage.service.write independently of read. Version must be
// the last observed note version (zero for a new note); conflicts return Aborted.
func (c *Client) PutNote(ctx context.Context, revision int64, note Note) (Note, error) {
	if revision <= 0 || note.Version < 0 || note.Version == math.MaxInt64 || len(note.Text) > 16384 || !utf8.ValidString(note.Text) {
		return Note{}, status.Error(codes.InvalidArgument, "positive revision, nonnegative version and UTF-8 text up to 16 KiB required")
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.gateway.PutNote(ctx, &v1.PutNoteRequest{Revision: revision, Version: note.Version, Text: note.Text})
	if err != nil {
		return Note{}, fmt.Errorf("put extension note: %w", err)
	}
	if response == nil || response.Version != note.Version+1 || response.Text != note.Text {
		return Note{}, status.Error(codes.DataLoss, "invalid extension note write response")
	}
	return Note{Text: response.Text, Version: response.Version}, nil
}
