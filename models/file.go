package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type File struct {
	ID           bson.ObjectID `bson:"_id,omitempty" json:"-"`
	PublicID     string        `bson:"public_id" json:"id"`
	OriginalName string        `bson:"original_name" json:"name"`
	StorageKey   string        `bson:"storage_key" json:"-"`
	ContentType  string        `bson:"content_type" json:"contentType"`
	Size         int64         `bson:"size" json:"size"`
	CreatedAt    time.Time     `bson:"created_at" json:"createdAt"`
	ExpiresAt    time.Time     `bson:"expires_at" json:"expiresAt"`
}
