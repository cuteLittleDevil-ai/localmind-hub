package memory

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Store persists agent sessions and messages (SPEC §5.2).
type Store struct {
	db *mongo.Database
}

type Session struct {
	SessionID string    `bson:"sessionId" json:"sessionId"`
	UserID    string    `bson:"userId" json:"userId"`
	Title     string    `bson:"title,omitempty" json:"title,omitempty"`
	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
}

type Message struct {
	SessionID string         `bson:"sessionId" json:"sessionId"`
	Role      string         `bson:"role" json:"role"`
	Content   string         `bson:"content,omitempty" json:"content,omitempty"`
	ToolName  string         `bson:"toolName,omitempty" json:"toolName,omitempty"`
	ToolArgs  map[string]any `bson:"toolArgs,omitempty" json:"toolArgs,omitempty"`
	CreatedAt time.Time      `bson:"createdAt" json:"createdAt"`
}

func NewStore(client *mongo.Client, database string) *Store {
	return &Store{db: client.Database(database)}
}

func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.db.Collection("sessions").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "sessionId", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}
	_, err = s.db.Collection("messages").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "sessionId", Value: 1}, {Key: "createdAt", Value: 1}},
	})
	return err
}

// GetOrCreateSession returns existing or new session id.
func (s *Store) GetOrCreateSession(ctx context.Context, sessionID, userID string) (string, error) {
	if sessionID != "" {
		var sess Session
		err := s.db.Collection("sessions").FindOne(ctx, bson.M{"sessionId": sessionID}).Decode(&sess)
		if err == nil {
			return sess.SessionID, nil
		}
		if err != mongo.ErrNoDocuments {
			return "", err
		}
	}
	sessionID = uuid.NewString()
	now := time.Now().UTC()
	sess := Session{
		SessionID: sessionID,
		UserID:    userID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := s.db.Collection("sessions").InsertOne(ctx, sess)
	return sessionID, err
}

func (s *Store) TouchSession(ctx context.Context, sessionID string) error {
	_, err := s.db.Collection("sessions").UpdateOne(ctx,
		bson.M{"sessionId": sessionID},
		bson.M{"$set": bson.M{"updatedAt": time.Now().UTC()}},
	)
	return err
}

func (s *Store) AppendMessage(ctx context.Context, msg *Message) error {
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.Collection("messages").InsertOne(ctx, msg)
	return err
}

func (s *Store) ListMessages(ctx context.Context, sessionID string, limit int64) ([]Message, error) {
	if limit <= 0 {
		limit = 100
	}
	cur, err := s.db.Collection("messages").Find(ctx,
		bson.M{"sessionId": sessionID},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}).SetLimit(limit),
	)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []Message
	for cur.Next(ctx) {
		var m Message
		if err := cur.Decode(&m); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, cur.Err()
}

func (s *Store) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	var sess Session
	err := s.db.Collection("sessions").FindOne(ctx, bson.M{"sessionId": sessionID}).Decode(&sess)
	if err != nil {
		return nil, err
	}
	return &sess, nil
}
