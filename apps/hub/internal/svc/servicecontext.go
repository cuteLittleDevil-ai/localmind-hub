package svc

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cuteLittleDevil-ai/localmind-hub/apps/hub/internal/config"
	"github.com/cuteLittleDevil-ai/localmind-hub/pkg/agent"
	"github.com/cuteLittleDevil-ai/localmind-hub/pkg/device"
	"github.com/cuteLittleDevil-ai/localmind-hub/pkg/llm"
	mem "github.com/cuteLittleDevil-ai/localmind-hub/pkg/memory/mongo"
	"github.com/cuteLittleDevil-ai/localmind-hub/pkg/mqttx"
	"github.com/cuteLittleDevil-ai/localmind-hub/pkg/store/sqlite"
	"github.com/cuteLittleDevil-ai/localmind-hub/pkg/tools"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ServiceContext struct {
	Config  config.Config
	Redis   *redis.Client
	Store   *sqlite.Store
	Mqtt    *mqttx.Client
	Devices *device.Service
	Mongo   *mongo.Client
	Memory  *mem.Store
	LLM     *llm.Client
	Agent   *agent.Orchestrator
}

func NewServiceContext(c config.Config) *ServiceContext {
	store, err := sqlite.Open(c.Data.SqlitePath)
	if err != nil {
		log.Fatalf("sqlite open: %v", err)
	}
	_ = store.EnsureDemoUser("demo")

	rdb := redis.NewClient(&redis.Options{Addr: c.Redis.Addr})
	mqttClient := mqttx.NewClient(mqttx.Config{Broker: c.Mqtt.Broker})
	devices := device.NewService(store, rdb, mqttClient)
	devices.StartMQTTHandlers()

	if err := mqttClient.Connect(); err != nil {
		log.Printf("mqtt connect (will retry via auto-reconnect): %v", err)
	} else {
		log.Printf("mqtt connected: %s", c.Mqtt.Broker)
	}

	llmClient := llm.NewClient(llm.Config{
		BaseURL: c.Ollama.BaseURL,
		Model:   c.Ollama.Model,
	})

	var mongoClient *mongo.Client
	var memStore *mem.Store
	var orchestrator *agent.Orchestrator

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	mongoClient, err = mongo.Connect(ctx, options.Client().ApplyURI(c.Mongo.URI))
	if err != nil {
		log.Printf("mongo connect: %v (agent chat will fail until mongo is up)", err)
	} else if err = mongoClient.Ping(ctx, nil); err != nil {
		log.Printf("mongo ping: %v", err)
	} else {
		memStore = mem.NewStore(mongoClient, c.Mongo.Database)
		if err := memStore.EnsureIndexes(ctx); err != nil {
			log.Printf("mongo indexes: %v", err)
		}
		reg := tools.NewRegistry(devices)
		orchestrator = agent.NewOrchestrator(llmClient, reg, memStore, agent.Config{
			RequireConfirmForControl: c.Agent.RequireConfirmForControl,
		})
	}

	return &ServiceContext{
		Config:  c,
		Redis:   rdb,
		Store:   store,
		Mqtt:    mqttClient,
		Devices: devices,
		Mongo:   mongoClient,
		Memory:  memStore,
		LLM:     llmClient,
		Agent:   orchestrator,
	}
}

// Close releases resources (best-effort for tests).
func (s *ServiceContext) Close() error {
	s.Mqtt.Disconnect()
	var err error
	if s.Mongo != nil {
		if e := s.Mongo.Disconnect(context.Background()); e != nil {
			err = fmt.Errorf("mongo: %w", e)
		}
	}
	if e := s.Store.Close(); e != nil {
		err = e
	}
	if e := s.Redis.Close(); e != nil {
		err = fmt.Errorf("%v; redis: %w", err, e)
	}
	return err
}
