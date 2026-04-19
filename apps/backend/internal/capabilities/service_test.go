package capabilities

import (
	"context"
	"testing"
)

func TestNewService(t *testing.T) {
	svc := NewService(nil)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestListTallyRules_WithoutCompute(t *testing.T) {
	svc := NewService(nil)

	items, err := svc.ListTallyRules(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "compute client unavailable" {
		t.Fatalf("unexpected error: %v", err)
	}
	if items != nil {
		t.Fatalf("expected nil items, got %#v", items)
	}
}