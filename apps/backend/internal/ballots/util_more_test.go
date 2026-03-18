package ballots

import (
	"strings"
	"testing"
	"time"
)

func TestComputeVoterHash_IsDeterministic(t *testing.T) {
	a := computeVoterHash("election-1", "user-1")
	b := computeVoterHash("election-1", "user-1")
	c := computeVoterHash("election-1", "user-2")

	if a != b {
		t.Fatalf("expected deterministic hash, got %q and %q", a, b)
	}
	if a == c {
		t.Fatalf("expected different hashes for different users, got %q", a)
	}
	if len(a) != 64 {
		t.Fatalf("expected sha256 hex length 64, got %d", len(a))
	}
	if strings.TrimSpace(a) == "" {
		t.Fatal("expected non-empty hash")
	}
}

func TestToJSONBOrNull(t *testing.T) {
	if got := toJSONBOrNull(nil); got != nil {
		t.Fatalf("expected nil, got %#v", got)
	}
	if got := toJSONBOrNull([]byte{}); got != nil {
		t.Fatalf("expected nil, got %#v", got)
	}
	if got := toJSONBOrNull([]byte(`{"a":1}`)); got != `{"a":1}` {
		t.Fatalf("unexpected jsonb conversion: %#v", got)
	}
}

func TestNewService(t *testing.T) {
	svc := NewService(nil, nil, 3*time.Minute)
	if svc == nil {
		t.Fatal("expected service")
	}
	if svc.idemTTL != 3*time.Minute {
		t.Fatalf("unexpected ttl: %v", svc.idemTTL)
	}
}
