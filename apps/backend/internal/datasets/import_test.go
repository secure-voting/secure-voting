package datasets

import (
	"context"
	"mime/multipart"
	"net/textproto"
	"os"
	"testing"
)

func TestImport_BallotsWithoutCandidates_ReturnsInvalidCandidatesBeforeDBAccess(t *testing.T) {
	svc := &Service{}

	tmp, err := os.CreateTemp("", "dataset-import-*.json")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()

	content := `{
		"dataset": {
			"name": "imported-dataset",
			"format": "ranking",
			"candidates": []
		},
		"ballots": [
			{ "voter_ref": "v1", "ranking": ["c1"] }
		]
	}`

	if _, err := tmp.WriteString(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		t.Fatalf("seek temp file: %v", err)
	}

	fh := &multipart.FileHeader{
		Filename: "dataset.json",
		Header: textproto.MIMEHeader{
			"Content-Type": []string{"application/json"},
		},
	}

	id, code, err := svc.Import(context.Background(), ImportMeta{}, fh, tmp)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if id != "" {
		t.Fatalf("expected empty id, got %q", id)
	}
	if code != "invalid_candidates" {
		t.Fatalf("expected invalid_candidates, got %q", code)
	}
}
