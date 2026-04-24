package main

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"secure-voting/apps/backend/internal/computeclient"
)

func connectMongo(ctx context.Context, cfg Config) (*mongo.Client, *mongo.Database, error) {
	mc, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		return nil, nil, err
	}
	return mc, mc.Database(cfg.MongoDB), nil
}

func connectPostgres(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func connectCompute(ctx context.Context, cfg Config) (*computeclient.Client, error) {
	cc, err := computeclient.New(ctx, computeclient.Config{
		Addr:       cfg.GRPCAddr,
		UseTLS:     cfg.UseTLS,
		CACertPath: cfg.CACertPath,
		ServerName: cfg.ServerName,
	})
	if err != nil {
		log.Printf("grpc dial: %v", err)
		return nil, err
	}
	return cc, nil
}
