package main

import (
	"bytes"
	"context"
	"testing"

	extension "github.com/TheModCraft-TMC/Vessel-HQ/sdk/go"
	v1 "github.com/TheModCraft-TMC/Vessel-HQ/sdk/go/proto/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeNotes struct {
	puts      int
	submitted extension.Note
	conflict  bool
}

func (*fakeNotes) GetRegistration(context.Context) (*v1.GetRegistrationResponse, error) {
	return &v1.GetRegistrationResponse{Revision: 7}, nil
}
func (*fakeNotes) GetNote(_ context.Context, revision int64) (extension.Note, error) {
	if revision != 7 {
		panic("wrong approval revision")
	}
	return extension.Note{Version: 3, Text: "private content"}, nil
}
func (f *fakeNotes) PutNote(_ context.Context, revision int64, note extension.Note) (extension.Note, error) {
	if revision != 7 {
		panic("wrong approval revision")
	}
	f.puts++
	f.submitted = note
	if f.conflict {
		return extension.Note{}, status.Error(codes.Aborted, "changed")
	}
	note.Version++
	return note, nil
}

func TestOmittedTextReadsAndExplicitEmptyTextClears(t *testing.T) {
	for _, test := range []struct {
		name      string
		arguments []string
		puts      int
		output    string
	}{{"read", nil, 0, "{\"version\":3,\"saved\":false}\n"}, {"clear", []string{"--text="}, 1, "{\"version\":4,\"saved\":true}\n"}} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			value, err := parseOptions(test.arguments, &output)
			if err != nil {
				t.Fatal(err)
			}
			client := &fakeNotes{}
			if err := execute(t.Context(), value, client, &output); err != nil {
				t.Fatal(err)
			}
			if client.puts != test.puts || output.String() != test.output {
				t.Fatalf("puts=%d output=%s", client.puts, &output)
			}
			if test.puts > 0 && (client.submitted.Version != 3 || client.submitted.Text != "") {
				t.Fatalf("lost CAS/empty text: %+v", client.submitted)
			}
		})
	}
}
func TestConflictDoesNotRetryOrPrintText(t *testing.T) {
	var output bytes.Buffer
	client := &fakeNotes{conflict: true}
	err := execute(t.Context(), options{update: true, text: "new private text"}, client, &output)
	if status.Code(err) != codes.Aborted || client.puts != 1 || output.Len() != 0 {
		t.Fatalf("err=%v puts=%d output=%s", err, client.puts, &output)
	}
}
func TestTextOutputRequiresExplicitOptIn(t *testing.T) {
	var output bytes.Buffer
	if err := execute(t.Context(), options{showText: true}, &fakeNotes{}, &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "{\"version\":3,\"saved\":false,\"text\":\"private content\"}\n" {
		t.Fatal(output.String())
	}
}
