package movies

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

const connectTimeout = 15 * time.Second

var ErrNotFound = errors.New("movie document not found")

type MovieRef struct {
	ID            primitive.ObjectID
	Caption       string
	FileName      string
	FileSize      int64
	MessageID     int
	SourceChannel int64
}

type Repository struct {
	client     *mongo.Client
	collection *mongo.Collection
	log        *zap.Logger
	enabled    bool
}

var repo = &Repository{}

func Init(log *zap.Logger, uri, database, collection string) error {
	repo = &Repository{log: log.Named("movies")}
	if uri == "" || database == "" {
		repo.log.Info("Mongo repository disabled")
		return nil
	}
	if collection == "" {
		collection = "movies"
	}
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return err
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return err
	}

	repo.client = client
	repo.collection = client.Database(database).Collection(collection)
	repo.enabled = true
	repo.log.Info("Mongo repository enabled", zap.String("database", database), zap.String("collection", collection))
	return nil
}

func Enabled() bool {
	return repo != nil && repo.enabled
}

func Close(ctx context.Context) error {
	if !Enabled() {
		return nil
	}
	return repo.client.Disconnect(ctx)
}

func FindByID(ctx context.Context, id string) (*MovieRef, error) {
	if !Enabled() {
		return nil, errors.New("mongo repository is disabled")
	}
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, fmt.Errorf("invalid mongo object id: %w", err)
	}

	var doc bson.M
	err = repo.collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	messageID, err := asInt64(doc["messageId"])
	if err != nil {
		return nil, fmt.Errorf("invalid messageId: %w", err)
	}
	sourceChannel, err := asInt64(doc["sourceChannel"])
	if err != nil {
		return nil, fmt.Errorf("invalid sourceChannel: %w", err)
	}
	fileSize := int64(0)
	if rawFileSize, ok := doc["fileSize"]; ok && rawFileSize != nil {
		fileSize, err = asInt64(rawFileSize)
		if err != nil {
			return nil, fmt.Errorf("invalid fileSize: %w", err)
		}
	}

	if messageID <= 0 {
		return nil, errors.New("messageId must be greater than 0")
	}

	return &MovieRef{
		ID:            objectID,
		Caption:       asString(doc["caption"]),
		FileName:      asString(doc["fileName"]),
		FileSize:      fileSize,
		MessageID:     int(messageID),
		SourceChannel: sourceChannel,
	}, nil
}

func asString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	default:
		return ""
	}
}

func asInt64(v any) (int64, error) {
	switch x := v.(type) {
	case int:
		return int64(x), nil
	case int32:
		return int64(x), nil
	case int64:
		return x, nil
	case float64:
		return int64(x), nil
	case string:
		return strconv.ParseInt(x, 10, 64)
	case primitive.Decimal128:
		return strconv.ParseInt(x.String(), 10, 64)
	case bson.M:
		return extractNumberLong(x)
	case map[string]any:
		return extractNumberLong(x)
	default:
		return 0, fmt.Errorf("unsupported numeric type %T", v)
	}
}

func extractNumberLong(m map[string]any) (int64, error) {
	v, ok := m["$numberLong"]
	if !ok {
		return 0, errors.New("missing $numberLong")
	}
	s, ok := v.(string)
	if !ok {
		return 0, errors.New("$numberLong must be string")
	}
	return strconv.ParseInt(s, 10, 64)
}
