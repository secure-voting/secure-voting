package datasets

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type fakeBallotCursor struct {
	items []BallotDoc
	idx   int
	err   error
}

func (c *fakeBallotCursor) Next(context.Context) bool {
	return c.idx < len(c.items)
}

func (c *fakeBallotCursor) Decode(v any) error {
	if c.idx >= len(c.items) {
		return errors.New("decode past end")
	}
	dst, ok := v.(*BallotDoc)
	if !ok {
		return errors.New("expected *BallotDoc")
	}
	*dst = c.items[c.idx]
	c.idx++
	return nil
}

func (c *fakeBallotCursor) Close(context.Context) error {
	return nil
}

func (c *fakeBallotCursor) Err() error {
	return c.err
}

func installDatasetExportHooks(doc DatasetDoc, ballots []BallotDoc) {
	datasetFindOneFn = func(_ context.Context, _ *mongo.Database, _ string, _ bson.M) singleResult {
		return fakeSingleResult{
			decodeFn: func(v any) error {
				dst := v.(*DatasetDoc)
				*dst = doc
				return nil
			},
		}
	}

	datasetFindFn = func(_ context.Context, _ *mongo.Database, collection string, _ bson.M, _ ...*options.FindOptions) (cursor, error) {
		if collection != "dataset_ballots" {
			return nil, errors.New("unexpected collection " + collection)
		}
		return &fakeBallotCursor{items: ballots}, nil
	}
}

func TestExportCSV_Ranking(t *testing.T) {
	defer restoreDatasetHooks()()

	doc := sampleDatasetDoc()
	doc.Name = "ranking dataset"
	doc.Format = "ranking"

	installDatasetExportHooks(doc, []BallotDoc{
		{VoterRef: "v1", Ranking: []string{"c1", "c2"}},
		{VoterRef: "v2", Ranking: []string{"c2", "c1"}},
	})

	svc := NewService(nil)
	data, filename, mime, code, err := svc.Export(context.Background(), doc.ID.Hex(), "csv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %q", code)
	}
	if filename != "ranking-dataset.csv" {
		t.Fatalf("unexpected filename: %q", filename)
	}
	if !strings.Contains(mime, "text/csv") {
		t.Fatalf("unexpected mime: %q", mime)
	}

	body := string(data)
	if !strings.Contains(body, "record_type,id,name,voter_ref,approval,ranking,scores") {
		t.Fatalf("missing header: %s", body)
	}
	if !strings.Contains(body, "candidate,c1,Alice,,,,") {
		t.Fatalf("missing candidate row: %s", body)
	}
	if !strings.Contains(body, "ballot,,,v1,,c1|c2,") {
		t.Fatalf("missing ranking row: %s", body)
	}
}

func TestExportTXT_Approval(t *testing.T) {
	defer restoreDatasetHooks()()

	doc := sampleDatasetDoc()
	doc.Name = "approval dataset"
	doc.Format = "approval"

	installDatasetExportHooks(doc, []BallotDoc{
		{VoterRef: "v1", Approval: []string{"c1", "c2"}},
	})

	svc := NewService(nil)
	data, filename, mime, code, err := svc.Export(context.Background(), doc.ID.Hex(), "txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %q", code)
	}
	if filename != "approval-dataset.txt" {
		t.Fatalf("unexpected filename: %q", filename)
	}
	if !strings.Contains(mime, "text/plain") {
		t.Fatalf("unexpected mime: %q", mime)
	}

	body := string(data)
	if !strings.Contains(body, "record_type\tid\tname\tvoter_ref\tapproval\tranking\tscores") {
		t.Fatalf("missing header: %s", body)
	}
	if !strings.Contains(body, "ballot\t\t\tv1\tc1|c2\t\t") {
		t.Fatalf("missing approval row: %s", body)
	}
}

func TestExportCSV_Score(t *testing.T) {
	defer restoreDatasetHooks()()

	doc := sampleDatasetDoc()
	doc.Name = "score dataset"
	doc.Format = "score"

	installDatasetExportHooks(doc, []BallotDoc{
		{VoterRef: "v1", Scores: map[string]int{"c1": 5, "c2": 3}},
	})

	svc := NewService(nil)
	data, filename, mime, code, err := svc.Export(context.Background(), doc.ID.Hex(), "csv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %q", code)
	}
	if filename != "score-dataset.csv" {
		t.Fatalf("unexpected filename: %q", filename)
	}
	if !strings.Contains(mime, "text/csv") {
		t.Fatalf("unexpected mime: %q", mime)
	}

	body := string(data)
	if !strings.Contains(body, "ballot,,,v1,,,c1=5|c2=3") {
		t.Fatalf("missing score row: %s", body)
	}
}

func TestExportXLSX_Ranking(t *testing.T) {
	defer restoreDatasetHooks()()

	doc := sampleDatasetDoc()
	doc.Name = "xlsx dataset"
	doc.Format = "ranking"

	installDatasetExportHooks(doc, []BallotDoc{
		{VoterRef: "v1", Ranking: []string{"c1", "c2"}},
	})

	svc := NewService(nil)
	data, filename, mime, code, err := svc.Export(context.Background(), doc.ID.Hex(), "xlsx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %q", code)
	}
	if filename != "xlsx-dataset.xlsx" {
		t.Fatalf("unexpected filename: %q", filename)
	}
	if !strings.Contains(mime, "spreadsheetml") {
		t.Fatalf("unexpected mime: %q", mime)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("invalid xlsx zip: %v", err)
	}

	foundSheet := false
	for _, f := range zr.File {
		if f.Name == "xl/worksheets/sheet1.xml" {
			foundSheet = true
			break
		}
	}
	if !foundSheet {
		t.Fatalf("xlsx sheet not found")
	}
}

func TestExportUnsupportedFormat(t *testing.T) {
	svc := NewService(nil)

	_, _, _, code, err := svc.Export(context.Background(), "bad", "pdf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "invalid_id" {
		t.Fatalf("expected invalid_id first, got %q", code)
	}

	doc := sampleDatasetDoc()
	installDatasetExportHooks(doc, nil)

	_, _, _, code, err = svc.Export(context.Background(), doc.ID.Hex(), "pdf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "unsupported_export_format" {
		t.Fatalf("expected unsupported_export_format, got %q", code)
	}
}

func TestExportNoBallots(t *testing.T) {
	defer restoreDatasetHooks()()

	doc := sampleDatasetDoc()
	installDatasetExportHooks(doc, nil)

	svc := NewService(nil)
	_, _, _, code, err := svc.Export(context.Background(), doc.ID.Hex(), "csv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "no_ballots" {
		t.Fatalf("expected no_ballots, got %q", code)
	}
}