package computeclient

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"

	pb "secure-voting/apps/backend/internal/compute/pb"

	"google.golang.org/grpc"
)

type dummyComputeServer struct {
	pb.UnimplementedComputeServer
}

func TestNew_InsecureSuccess(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer lis.Close()

	srv := grpc.NewServer()
	pb.RegisterComputeServer(srv, &dummyComputeServer{})
	defer srv.Stop()

	go func() {
		_ = srv.Serve(lis)
	}()

	client, err := New(context.Background(), Config{
		Addr:   lis.Addr().String(),
		UseTLS: false,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if client == nil {
		t.Fatal("expected client")
	}
	if client.Compute() == nil {
		t.Fatal("expected non-nil compute client")
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
}

func TestNew_TLSMissingCA(t *testing.T) {
	_, err := New(context.Background(), Config{
		Addr:       "127.0.0.1:65535",
		UseTLS:     true,
		CACertPath: filepath.Join(t.TempDir(), "missing.pem"),
		ServerName: "localhost",
	})
	if err == nil {
		t.Fatal("expected error for missing CA")
	}
}

func TestNew_TLSInvalidPEM(t *testing.T) {
	dir := t.TempDir()
	caPath := filepath.Join(dir, "bad.pem")
	if err := os.WriteFile(caPath, []byte("not-a-pem"), 0o600); err != nil {
		t.Fatalf("write temp pem: %v", err)
	}

	_, err := New(context.Background(), Config{
		Addr:       "127.0.0.1:65535",
		UseTLS:     true,
		CACertPath: caPath,
		ServerName: "localhost",
	})
	if !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("expected os.ErrInvalid, got %v", err)
	}
}

func TestCloseNilConn(t *testing.T) {
	c := &Client{}
	if err := c.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestComputeGetter(t *testing.T) {
	c := &Client{}
	if c.Compute() != nil {
		t.Fatal("expected nil compute client for zero value")
	}
}
