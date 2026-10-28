package memory

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 事件类型与 SPEC §5.2 events 集合一致。
const (
	EventTypeDiagnose       = "diagnose"
	EventTypeOfflineTimeout = "offline_timeout"
	EventTypeControlFail    = "control_fail"
)

// Store 持久化 Agent 会话、消息与设备诊断/控制事件（Mongo）。
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

// Event 设备侧诊断与控制失败记录（events 集合）。
type Event struct {
	ID        string         `bson:"_id,omitempty" json:"id,omitempty"`
	DeviceID  string         `bson:"deviceId" json:"deviceId"`
	Type      string         `bson:"type" json:"type"`
	Payload   map[string]any `bson:"payload" json:"payload"`
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
	if err != nil {
		return err
	}
	_, err = s.db.Collection("events").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "deviceId", Value: 1}, {Key: "createdAt", Value: -1}},
	})
	return err
}

// InsertEvent 写入设备事件，返回 Mongo ObjectId 十六进制字符串。
func (s *Store) InsertEvent(ctx context.Context, deviceID, eventType string, payload map[string]any) (string, error) {
	if s == nil || s.db == nil {
		return "", mongo.ErrClientDisconnected
	}
	if payload == nil {
		payload = map[string]any{}
	}
	doc := Event{
		DeviceID:  deviceID,
		Type:      eventType,
		Payload:   payload,
		CreatedAt: time.Now().UTC(),
	}
	res, err := s.db.Collection("events").InsertOne(ctx, doc)
	if err != nil {
		return "", err
	}
	oid, ok := res.InsertedID.(primitive.ObjectID)
	if !ok {
		return "", nil
	}
	return oid.Hex(), nil
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
