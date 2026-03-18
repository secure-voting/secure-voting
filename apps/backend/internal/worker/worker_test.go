package worker

import (
	"testing"
	"time"
)

func TestNewAndClose(t *testing.T) {
	w := New(nil, nil, Config{
		PollInterval: time.Second,
		TasksTopic:   "tasks",
		ResultsTopic: "results",
		GroupID:      "group",
		Brokers:      []string{"localhost:9092"},
	})
	if w == nil {
		t.Fatal("expected worker")
	}
	if w.kw == nil {
		t.Fatal("expected kafka writer")
	}
	if w.kr == nil {
		t.Fatal("expected kafka reader")
	}
	w.Close()
}

func TestNew_DefaultPollInterval(t *testing.T) {
	w := New(nil, nil, Config{
		TasksTopic:   "tasks",
		ResultsTopic: "results",
		GroupID:      "group",
		Brokers:      []string{"localhost:9092"},
	})
	if w == nil {
		t.Fatal("expected worker")
	}
	if w.pollInterval <= 0 {
		t.Fatalf("expected positive poll interval, got %v", w.pollInterval)
	}
	w.Close()
}
