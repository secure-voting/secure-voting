package notifications

import (
	"context"
	"testing"
)

func TestNormalizeKind(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"info", "info"},
		{"success", "success"},
		{"warning", "warning"},
		{"error", "error"},
		{"unknown", "info"},
		{"", "info"},
	}

	for _, tc := range cases {
		if got := normalizeKind(tc.in); got != tc.want {
			t.Fatalf("normalizeKind(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestValidationHelpers(t *testing.T) {
	if !validateTitle("x") {
		t.Fatal("expected valid title")
	}
	if validateTitle("") {
		t.Fatal("expected invalid empty title")
	}
	if !validateMessage("x") {
		t.Fatal("expected valid message")
	}
	if validateMessage("") {
		t.Fatal("expected invalid empty message")
	}
	if !validateDetails("") {
		t.Fatal("expected valid empty details")
	}
	if !validateActionLabel("") {
		t.Fatal("expected valid empty action label")
	}
	if !validateActionTo("") {
		t.Fatal("expected valid empty action_to")
	}
}

func TestCreate_ValidationOnly(t *testing.T) {
	svc := NewService(nil)

	if _, code, err := svc.Create(context.Background(), CreateInput{}); err != nil || code != "unauthorized" {
		t.Fatalf("expected unauthorized, got code=%q err=%v", code, err)
	}

	if _, code, err := svc.Create(context.Background(), CreateInput{
		UserID:  "u1",
		Title:   "",
		Message: "m",
	}); err != nil || code != "invalid_title" {
		t.Fatalf("expected invalid_title, got code=%q err=%v", code, err)
	}

	if _, code, err := svc.Create(context.Background(), CreateInput{
		UserID:  "u1",
		Title:   "t",
		Message: "",
	}); err != nil || code != "invalid_message" {
		t.Fatalf("expected invalid_message, got code=%q err=%v", code, err)
	}
}

func TestList_ValidationOnly(t *testing.T) {
	svc := NewService(nil)
	items, code, err := svc.List(context.Background(), "", 10, 0)
	if err != nil || code != "unauthorized" || items != nil {
		t.Fatalf("expected unauthorized, got items=%v code=%q err=%v", items, code, err)
	}
}

func TestMarkRead_ValidationOnly(t *testing.T) {
	svc := NewService(nil)

	if code, err := svc.MarkRead(context.Background(), "", "n1"); err != nil || code != "unauthorized" {
		t.Fatalf("expected unauthorized, got code=%q err=%v", code, err)
	}

	if code, err := svc.MarkRead(context.Background(), "u1", ""); err != nil || code != "invalid_id" {
		t.Fatalf("expected invalid_id, got code=%q err=%v", code, err)
	}
}

func TestMarkAllRead_ValidationOnly(t *testing.T) {
	svc := NewService(nil)
	if code, err := svc.MarkAllRead(context.Background(), ""); err != nil || code != "unauthorized" {
		t.Fatalf("expected unauthorized, got code=%q err=%v", code, err)
	}
}

func TestDelete_ValidationOnly(t *testing.T) {
	svc := NewService(nil)

	if code, err := svc.Delete(context.Background(), "", "n1"); err != nil || code != "unauthorized" {
		t.Fatalf("expected unauthorized, got code=%q err=%v", code, err)
	}

	if code, err := svc.Delete(context.Background(), "u1", ""); err != nil || code != "invalid_id" {
		t.Fatalf("expected invalid_id, got code=%q err=%v", code, err)
	}
}

func TestClearAll_ValidationOnly(t *testing.T) {
	svc := NewService(nil)
	if code, err := svc.ClearAll(context.Background(), ""); err != nil || code != "unauthorized" {
		t.Fatalf("expected unauthorized, got code=%q err=%v", code, err)
	}
}

func TestSeedIfEmpty_EmptyUserID(t *testing.T) {
	svc := NewService(nil)
	if err := svc.SeedIfEmpty(context.Background(), ""); err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
}
