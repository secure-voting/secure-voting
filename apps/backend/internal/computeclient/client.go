package computeclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"
	"time"

	pb "secure-voting/apps/backend/internal/compute/pb"

	"google.golang.org/grpc"
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

	conn, err := grpc.DialContext(
		ctx,
		cfg.Addr,
		transportOpt,
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, err
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
