package svc

import (
	"fmt"
	"log"

	"github.com/cuteLittleDevil-ai/localmind-hub/apps/hub/internal/config"
	"github.com/cuteLittleDevil-ai/localmind-hub/pkg/device"
	"github.com/cuteLittleDevil-ai/localmind-hub/pkg/mqttx"
	"github.com/cuteLittleDevil-ai/localmind-hub/pkg/store/sqlite"
	"github.com/redis/go-redis/v9"
)

type ServiceContext struct {
	Config  config.Config
	Redis   *redis.Client
	Store   *sqlite.Store
	Mqtt    *mqttx.Client
	Devices *device.Service
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

	return &ServiceContext{
		Config:  c,
		Redis:   rdb,
		Store:   store,
		Mqtt:    mqttClient,
		Devices: devices,
	}
}

// Close releases resources (best-effort for tests).
func (s *ServiceContext) Close() error {
	s.Mqtt.Disconnect()
	var err error
	if e := s.Store.Close(); e != nil {
		err = e
	}
	if e := s.Redis.Close(); e != nil {
		err = fmt.Errorf("%v; redis: %w", err, e)
	}
	return err
}
