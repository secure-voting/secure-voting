package db

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

func NewMongoClient(ctx context.Context, uri string) (*mongo.Client, error) {
	cctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	client, err := mongo.Connect(cctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	pctx, cancel2 := context.WithTimeout(ctx, 5*time.Second)
	defer cancel2()

	if err := client.Ping(pctx, readpref.Primary()); err != nil {
		_ = client.Disconnect(ctx)
		return nil, err
	}
	return client, nil
}
