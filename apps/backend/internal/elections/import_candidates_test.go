package elections

import (
	"context"
	"testing"
)

func TestImportCandidatesCSV(t *testing.T) {
	svc := &Service{}

	items, code, err := svc.ImportCandidates(context.Background(), "candidates.csv", []byte(
		"name,description\n Иванов Иван Иванович , Кандидат 1 \nПетров Петр Петрович,\n",
	))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %s", code)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Name != "Иванов Иван Иванович" {
		t.Fatalf("unexpected first name: %q", items[0].Name)
	}
	if got, _ := items[0].Meta["description"].(string); got != "Кандидат 1" {
		t.Fatalf("unexpected description: %q", got)
	}
}

func TestImportCandidatesJSONObjects(t *testing.T) {
	svc := &Service{}

	items, code, err := svc.ImportCandidates(context.Background(), "candidates.json", []byte(`
[
  {"name":"Иванов Иван Иванович","description":"Кандидат 1"},
  {"name":"Петров Петр Петрович","meta":{"description":"Кандидат 2"}}
]
`))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %s", code)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if got, _ := items[1].Meta["description"].(string); got != "Кандидат 2" {
		t.Fatalf("unexpected description: %q", got)
	}
}

func TestImportCandidatesJSONStrings(t *testing.T) {
	svc := &Service{}

	items, code, err := svc.ImportCandidates(context.Background(), "candidates.json", []byte(`
["Иванов Иван Иванович","Петров Петр Петрович"]
`))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %s", code)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
}

func TestImportCandidatesDuplicate(t *testing.T) {
	svc := &Service{}

	_, code, err := svc.ImportCandidates(context.Background(), "candidates.json", []byte(`
["Иванов Иван Иванович","Иванов   Иван   Иванович"]
`))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if code != "duplicate_candidate_name" {
		t.Fatalf("unexpected code: %s", code)
	}
}

func TestImportCandidatesUnsupportedFormat(t *testing.T) {
	svc := &Service{}

	_, code, err := svc.ImportCandidates(context.Background(), "candidates.txt", []byte("a\nb\n"))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if code != "unsupported_file_format" {
		t.Fatalf("unexpected code: %s", code)
	}
}
