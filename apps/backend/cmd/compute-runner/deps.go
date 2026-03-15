package main

import (
	"context"
	"log"

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
