package computeclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	pb "secure-voting/apps/backend/internal/compute/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type Config struct {
	Addr       string
	UseTLS     bool
	CACertPath string
	ServerName string
}

type Client struct {
	conn   *grpc.ClientConn
	client pb.ComputeClient
}

func New(ctx context.Context, cfg Config) (*Client, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var transportOpt grpc.DialOption

	if cfg.UseTLS {
		caPEM, err := os.ReadFile(cfg.CACertPath)
		if err != nil {
			return nil, err
		}

		pool := x509.NewCertPool()
		if ok := pool.AppendCertsFromPEM(caPEM); !ok {
			return nil, os.ErrInvalid
		}

		tlsCfg := &tls.Config{
			MinVersion: tls.VersionTLS12,
			RootCAs:    pool,
			ServerName: cfg.ServerName,
		}
		transportOpt = grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg))
	} else {
		transportOpt = grpc.WithTransportCredentials(insecure.NewCredentials())
	}

	conn, err := grpc.NewClient(cfg.Addr, transportOpt)
	if err != nil {
		return nil, err
	}

	conn.Connect()

	for {
		state := conn.GetState()

		if state == connectivity.Ready {
			break
		}

		if state == connectivity.Shutdown {
			_ = conn.Close()
			return nil, fmt.Errorf("gRPC connection to %s entered shutdown state", cfg.Addr)
		}

		if !conn.WaitForStateChange(ctx, state) {
			_ = conn.Close()
			return nil, fmt.Errorf("gRPC connection to %s was not ready before timeout: %w", cfg.Addr, ctx.Err())
		}
	}

	return &Client{
		conn:   conn,
		client: pb.NewComputeClient(conn),
	}, nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) Compute() pb.ComputeClient {
	return c.client
}

func (c *Client) ConnectivityState() string {
	if c == nil || c.conn == nil {
		return connectivity.Shutdown.String()
	}
	return c.conn.GetState().String()
}

func (c *Client) Ready() bool {
	if c == nil || c.conn == nil {
		return false
	}
	return c.conn.GetState() == connectivity.Ready
}