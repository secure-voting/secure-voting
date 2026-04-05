package datasets

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type fakeSingleResult struct {
	decodeFn func(v any) error
}

func (r fakeSingleResult) Decode(v any) error {
	return r.decodeFn(v)
}

type fakeCursor struct {
	items []DatasetDoc
	idx   int
	err   error
}

func (c *fakeCursor) Next(context.Context) bool {
	return c.idx < len(c.items)
}

func (c *fakeCursor) Decode(v any) error {
	if c.idx >= len(c.items) {
		return errors.New("decode past end")
	}
	doc, ok := v.(*DatasetDoc)
	if !ok {
		return errors.New("expected *DatasetDoc")
	}
	*doc = c.items[c.idx]
	c.idx++
	return nil
}

func (c *fakeCursor) Close(context.Context) error { return nil }

func (c *fakeCursor) Err() error { return c.err }

func restoreDatasetHooks() func() {
	oldFindOne := datasetFindOneFn
	oldFind := datasetFindFn
	return func() {
		datasetFindOneFn = oldFindOne
		datasetFindFn = oldFind
	}
}

func sampleDatasetDoc() DatasetDoc {
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	seed := int64(42)
	return DatasetDoc{
		ID:          primitive.NewObjectID(),
		Name:        "dataset-1",
		Description: "desc",
		Source:      "generate",
		Format:      "ranking",
		Candidates: []Candidate{
			{ID: "c1", Name: "Alice"},
			{ID: "c2", Name: "Bob"},
		},
		CreatedAt:  now,
		Seed:       &seed,
		Parameters: map[string]any{"p": 1},
	}
}

func TestNewService(t *testing.T) {
	svc := NewService(nil)
	if svc == nil {
		t.Fatal("expected service")
	}
}

func TestGet_InvalidID(t *testing.T) {
	svc := NewService(nil)

	_, code, err := svc.Get(context.Background(), "bad")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "invalid_id" {
		t.Fatalf("expected invalid_id, got %q", code)
	}
}

func TestGet_NotFound(t *testing.T) {
	defer restoreDatasetHooks()()

	datasetFindOneFn = func(_ context.Context, _ *mongo.Database, _ string, _ primitive.M) singleResult {
		return fakeSingleResult{decodeFn: func(v any) error { return mongo.ErrNoDocuments }}
	}

	svc := NewService(nil)
	_, code, err := svc.Get(context.Background(), primitive.NewObjectID().Hex())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "not_found" {
		t.Fatalf("expected not_found, got %q", code)
	}
}

func TestGet_InternalError(t *testing.T) {
	defer restoreDatasetHooks()()

	datasetFindOneFn = func(_ context.Context, _ *mongo.Database, _ string, _ primitive.M) singleResult {
		return fakeSingleResult{decodeFn: func(v any) error { return errors.New("boom") }}
	}

	svc := NewService(nil)
	_, code, err := svc.Get(context.Background(), primitive.NewObjectID().Hex())
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected boom, got code=%q err=%v", code, err)
	}
}

func TestGet_Success(t *testing.T) {
	defer restoreDatasetHooks()()

	doc := sampleDatasetDoc()
	datasetFindOneFn = func(_ context.Context, _ *mongo.Database, _ string, _ primitive.M) singleResult {
		return fakeSingleResult{
			decodeFn: func(v any) error {
				dst := v.(*DatasetDoc)
				*dst = doc
				return nil
			},
		}
	}

	svc := NewService(nil)
	got, code, err := svc.Get(context.Background(), doc.ID.Hex())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %q", code)
	}
	if got.ID != doc.ID.Hex() || got.Name != doc.Name || got.Format != doc.Format {
		t.Fatalf("unexpected dataset: %#v", got)
	}
	if got.Seed == nil || *got.Seed != 42 {
		t.Fatalf("unexpected seed: %#v", got.Seed)
	}
}

func TestDownload_InvalidID(t *testing.T) {
	svc := NewService(nil)

	_, _, _, code, err := svc.Download(context.Background(), "bad")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "invalid_id" {
		t.Fatalf("expected invalid_id, got %q", code)
	}
}

func TestDownload_NotFound(t *testing.T) {
	defer restoreDatasetHooks()()

	datasetFindOneFn = func(_ context.Context, _ *mongo.Database, _ string, _ primitive.M) singleResult {
		return fakeSingleResult{decodeFn: func(v any) error { return mongo.ErrNoDocuments }}
	}

	svc := NewService(nil)
	_, _, _, code, err := svc.Download(context.Background(), primitive.NewObjectID().Hex())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "not_found" {
		t.Fatalf("expected not_found, got %q", code)
	}
}

func TestDownload_InternalError(t *testing.T) {
	defer restoreDatasetHooks()()

	datasetFindOneFn = func(_ context.Context, _ *mongo.Database, _ string, _ primitive.M) singleResult {
		return fakeSingleResult{decodeFn: func(v any) error { return errors.New("boom") }}
	}

	svc := NewService(nil)
	_, _, _, code, err := svc.Download(context.Background(), primitive.NewObjectID().Hex())
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected boom, got code=%q err=%v", code, err)
	}
}

func TestDownload_FallbackJSON(t *testing.T) {
	defer restoreDatasetHooks()()

	doc := sampleDatasetDoc()
	doc.Raw.Data = nil

	datasetFindOneFn = func(_ context.Context, _ *mongo.Database, _ string, _ primitive.M) singleResult {
		return fakeSingleResult{
			decodeFn: func(v any) error {
				dst := v.(*DatasetDoc)
				*dst = doc
				return nil
			},
		}
	}

	svc := NewService(nil)
	data, filename, mime, code, err := svc.Download(context.Background(), doc.ID.Hex())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %q", code)
	}
	if filename != "dataset.json" || mime != "application/json" {
		t.Fatalf("unexpected file meta: %q %q", filename, mime)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if payload["name"] != doc.Name {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestDownload_RawDataWithDefaults(t *testing.T) {
	defer restoreDatasetHooks()()

	doc := sampleDatasetDoc()
	doc.Raw.Data = []byte("abc")
	doc.RawFilename = ""
	doc.RawMime = ""

	datasetFindOneFn = func(_ context.Context, _ *mongo.Database, _ string, _ primitive.M) singleResult {
		return fakeSingleResult{
			decodeFn: func(v any) error {
				dst := v.(*DatasetDoc)
				*dst = doc
				return nil
			},
		}
	}

	svc := NewService(nil)
	data, filename, mime, code, err := svc.Download(context.Background(), doc.ID.Hex())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %q", code)
	}
	if string(data) != "abc" {
		t.Fatalf("unexpected data: %q", string(data))
	}
	if filename != "dataset.bin" {
		t.Fatalf("unexpected filename: %q", filename)
	}
	if mime != "application/octet-stream" {
		t.Fatalf("unexpected mime: %q", mime)
	}
}

func TestDownload_RawDataCustomMeta(t *testing.T) {
	defer restoreDatasetHooks()()

	doc := sampleDatasetDoc()
	doc.Raw.Data = []byte("xyz")
	doc.RawFilename = "custom.dat"
	doc.RawMime = "application/custom"

	datasetFindOneFn = func(_ context.Context, _ *mongo.Database, _ string, _ primitive.M) singleResult {
		return fakeSingleResult{
			decodeFn: func(v any) error {
				dst := v.(*DatasetDoc)
				*dst = doc
				return nil
			},
		}
	}

	svc := NewService(nil)
	_, filename, mime, code, err := svc.Download(context.Background(), doc.ID.Hex())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %q", code)
	}
	if filename != "custom.dat" || mime != "application/custom" {
		t.Fatalf("unexpected meta: %q %q", filename, mime)
	}
}

func TestList_FindError(t *testing.T) {
	defer restoreDatasetHooks()()

	datasetFindFn = func(_ context.Context, _ *mongo.Database, _ string, _ primitive.M, _ ...*options.FindOptions) (cursor, error) {
		return nil, errors.New("boom")
	}

	svc := NewService(nil)
	_, err := svc.List(context.Background())
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected boom, got %v", err)
	}
}

func TestList_CursorErr(t *testing.T) {
	defer restoreDatasetHooks()()

	datasetFindFn = func(_ context.Context, _ *mongo.Database, _ string, _ primitive.M, _ ...*options.FindOptions) (cursor, error) {
		return &fakeCursor{err: errors.New("cursor boom")}, nil
	}

	svc := NewService(nil)
	_, err := svc.List(context.Background())
	if err == nil || !strings.Contains(err.Error(), "cursor boom") {
		t.Fatalf("expected cursor boom, got %v", err)
	}
}

func TestList_Success(t *testing.T) {
	defer restoreDatasetHooks()()

	doc1 := sampleDatasetDoc()
	doc2 := sampleDatasetDoc()
	doc2.ID = primitive.NewObjectID()
	doc2.Name = "dataset-2"

	datasetFindFn = func(_ context.Context, _ *mongo.Database, _ string, _ primitive.M, _ ...*options.FindOptions) (cursor, error) {
		return &fakeCursor{items: []DatasetDoc{doc1, doc2}}, nil
	}

	svc := NewService(nil)
	items, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Name != "dataset-1" || items[1].Name != "dataset-2" {
		t.Fatalf("unexpected items: %#v", items)
	}
}
