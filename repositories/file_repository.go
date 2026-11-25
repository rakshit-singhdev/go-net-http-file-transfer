package repositories

import (
	"context"
	"errors"
	"file-sharing-server/models"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type FileRepository struct {
	collection *mongo.Collection
}

func NewFileRepository(db *mongo.Database) *FileRepository {
	return &FileRepository{
		collection: db.Collection("files"),
	}
}

func (r *FileRepository) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "public_id", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "expires_at", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "created_at", Value: -1},
			},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	return err
}

func (r *FileRepository) Create(
	ctx context.Context,
	file models.File,
) error {
	_, err := r.collection.InsertOne(ctx, file)
	return err
}

func (r *FileRepository) FindByPublicID(
	ctx context.Context,
	publicID string,
) (*models.File, error) {
	var file models.File

	err := r.collection.FindOne(
		ctx,
		bson.D{{Key: "public_id", Value: publicID}},
	).Decode(&file)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}

	return &file, nil
}

func (r *FileRepository) List(
    ctx context.Context,
    limit int64,
    skip int64,
) ([]models.File, error) {

    if limit <= 0 || limit > 100 {
        limit = 50
    }

    if skip < 0 {
        skip = 0
    }

    filter := bson.D{
        {
            Key: "expires_at",
            Value: bson.D{
                {Key: "$gt", Value: time.Now().UTC()},
            },
        },
    }

    opts := options.Find().
        SetSort(bson.D{{Key: "created_at", Value: -1}}).
        SetLimit(limit).
        SetSkip(skip)

    cursor, err := r.collection.Find(ctx, filter, opts)
    if err != nil {
        return nil, err
    }

    defer cursor.Close(ctx)

    files := make([]models.File, 0)

    if err := cursor.All(ctx, &files); err != nil {
        return nil, err
    }

    return files, nil
}

// Delete file metadata using its public ID.
func (r *FileRepository) DeleteByPublicID(
	ctx context.Context,
	publicID string,
) (bool, error) {
	result, err := r.collection.DeleteOne(
		ctx,
		bson.D{{Key: "public_id", Value: publicID}},
	)

	if err != nil {
		return false, err
	}

	return result.DeletedCount > 0, nil
}
