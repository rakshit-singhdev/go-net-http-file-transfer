package repositories

import (
	"context"
	"errors"
	"file-sharing-server/models"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type UserRepository struct {
    collection *mongo.Collection
}
func NewUserRepository(db *mongo.Database) *UserRepository {
	return &UserRepository{
		collection: db.Collection("users"),
	}
}

func (r *UserRepository) FindByid(
	ctx context.Context,
	id bson.ObjectID,
) (*models.User, error) {
	var user models.User

	err := r.collection.FindOne(
		ctx,
		bson.D{{Key: "_id", Value: id}},
	).Decode(&user)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments){
			return nil, nil
		}
		return nil, err
	}

	return &user, nil
}


func (r *UserRepository) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "email", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	return err
}

func (r *UserRepository) Create(
	ctx context.Context,
	user models.User,
) error {
	_, err := r.collection.InsertOne(ctx, user)
	return err
}

func (r *UserRepository) FindByEmail(
	ctx context.Context,
	email string,
) (*models.User, error) {
	var user models.User

	err := r.collection.FindOne(
		ctx,
		bson.D{{Key: "email", Value: email}},
	).Decode(&user)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}

	return &user, nil
}
