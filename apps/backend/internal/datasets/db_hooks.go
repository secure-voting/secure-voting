package datasets

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type singleResult interface {
	Decode(v any) error
}

type cursor interface {
	Next(context.Context) bool
	Decode(v any) error
	Close(context.Context) error
	Err() error
}

var datasetFindOneFn = func(ctx context.Context, db *mongo.Database, collection string, filter bson.M) singleResult {
	return db.Collection(collection).FindOne(ctx, filter)
}

var datasetFindFn = func(ctx context.Context, db *mongo.Database, collection string, filter bson.M, opts ...*options.FindOptions) (cursor, error) {
	return db.Collection(collection).Find(ctx, filter, opts...)
}
