package logic

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cuteLittleDevil-ai/localmind-hub/apps/hub/internal/svc"
	"github.com/cuteLittleDevil-ai/localmind-hub/apps/hub/internal/types"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type HealthLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewHealthLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HealthLogic {
	return &HealthLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *HealthLogic) Health() (*types.HealthResp, error) {
	checks := map[string]types.Check{
		"redis":  l.checkRedis(),
		"mongo":  l.checkMongo(),
		"mqtt":   l.checkMQTT(),
		"sqlite": l.checkSQLiteDataDir(),
	}

	status := "ok"
	for _, c := range checks {
		if !c.OK {
			status = "degraded"
			break
		}
	}

	return &types.HealthResp{Status: status, Checks: checks}, nil
}

func (l *HealthLogic) checkRedis() types.Check {
	ctx, cancel := context.WithTimeout(l.ctx, 2*time.Second)
	defer cancel()

	if err := l.svcCtx.Redis.Ping(ctx).Err(); err != nil {
		return types.Check{OK: false, Message: err.Error()}
	}
	return types.Check{OK: true, Message: "pong"}
}

func (l *HealthLogic) checkMongo() types.Check {
	ctx, cancel := context.WithTimeout(l.ctx, 3*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(l.svcCtx.Config.Mongo.URI))
	if err != nil {
		return types.Check{OK: false, Message: err.Error()}
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	if err := client.Ping(ctx, nil); err != nil {
		return types.Check{OK: false, Message: err.Error()}
	}
	return types.Check{OK: true, Message: "ping ok"}
}

func (l *HealthLogic) checkMQTT() types.Check {
	if l.svcCtx.Mqtt != nil && l.svcCtx.Mqtt.IsConnected() {
		return types.Check{OK: true, Message: "connected"}
	}
	broker := l.svcCtx.Config.Mqtt.Broker
	addr := strings.TrimPrefix(broker, "tcp://")
	addr = strings.TrimPrefix(addr, "mqtt://")
	if addr == "" {
		return types.Check{OK: false, Message: "empty mqtt broker"}
	}

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return types.Check{OK: false, Message: err.Error()}
	}
	_ = conn.Close()
	return types.Check{OK: true, Message: "tcp reachable"}
}

func (l *HealthLogic) checkSQLiteDataDir() types.Check {
	dir := filepath.Dir(l.svcCtx.Config.Data.SqlitePath)
	if dir == "" || dir == "." {
		dir = "data"
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return types.Check{OK: false, Message: err.Error()}
	}

	testFile := filepath.Join(dir, ".write_probe")
	if err := os.WriteFile(testFile, []byte("ok"), 0o644); err != nil {
		return types.Check{OK: false, Message: err.Error()}
	}
	_ = os.Remove(testFile)

	return types.Check{OK: true, Message: "data dir writable"}
}
