package worker

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/segmentio/kafka-go"
)

func mustKafkaTLSConfig(enabled bool, caPath, serverName string) *tls.Config {
	if !enabled {
		return nil
	}
	if caPath == "" {
		panic("kafka tls enabled but ca path is empty")
	}

	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		panic(fmt.Sprintf("read kafka ca failed: %v", err))
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		panic("append kafka ca failed")
	}

	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    pool,
		ServerName: serverName,
	}
}

func newKafkaWriter(cfg Config) *kafka.Writer {
	tlsCfg := mustKafkaTLSConfig(cfg.KafkaTLS, cfg.KafkaTLSCA, cfg.KafkaTLSServerName)

	return &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        cfg.TasksTopic,
		RequiredAcks: kafka.RequireAll,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 50 * time.Millisecond,
		Transport: &kafka.Transport{
			TLS: tlsCfg,
		},
	}
}

func newKafkaReader(cfg Config) *kafka.Reader {
	tlsCfg := mustKafkaTLSConfig(cfg.KafkaTLS, cfg.KafkaTLSCA, cfg.KafkaTLSServerName)

	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.Brokers,
		GroupID:        cfg.GroupID,
		Topic:          cfg.ResultsTopic,
		MinBytes:       1e3,
		MaxBytes:       10e6,
		MaxWait:        250 * time.Millisecond,
		StartOffset:    kafka.FirstOffset,
		CommitInterval: 0,
		Dialer: &kafka.Dialer{
			Timeout:   10 * time.Second,
			DualStack: true,
			TLS:       tlsCfg,
		},
	})
}
