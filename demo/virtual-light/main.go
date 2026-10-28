// Virtual light simulates a demo MQTT device (M1-7): handles down, sends ack and up.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/cuteLittleDevil-ai/localmind-hub/pkg/mqttx"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	broker := flag.String("broker", envOr("MQTT_BROKER", "tcp://127.0.0.1:1883"), "MQTT broker URL")
	deviceID := flag.String("device", envOr("DEVICE_ID", "light-living-01"), "device id")
	room := flag.String("room", envOr("ROOM", "living"), "room name")
	interval := flag.Duration("interval", 30*time.Second, "periodic up interval")
	flag.Parse()

	var mu sync.Mutex
	state := map[string]any{
		"online":     true,
		"brightness": float64(50),
		"room":       *room,
	}

	opts := mqtt.NewClientOptions().
		AddBroker(*broker).
		SetClientID(fmt.Sprintf("virtual-light-%s", *deviceID)).
		SetAutoReconnect(true)

	var client mqtt.Client
	opts.SetDefaultPublishHandler(func(_ mqtt.Client, msg mqtt.Message) {
		down, err := mqttx.DecodeDown(msg.Payload())
		if err != nil {
			log.Printf("bad down: %v", err)
			return
		}
		mu.Lock()
		if b, ok := down.Params["brightness"].(float64); ok {
			state["brightness"] = b
		}
		brightness := state["brightness"]
		mu.Unlock()

		ack := &mqttx.AckMessage{
			CmdID:          down.CmdID,
			IdempotencyKey: down.IdempotencyKey,
			OK:             true,
			Code:           0,
			Message:        "ok",
			Shadow:         map[string]any{"brightness": brightness, "room": *room, "online": true},
		}
		ackPayload, _ := mqttx.EncodeAck(ack)
		client.Publish(mqttx.TopicAck(*deviceID), 1, false, ackPayload)
		publishUp(client, *deviceID, state, &mu)
	})

	client = mqtt.NewClient(opts)
	token := client.Connect()
	if token.Wait() && token.Error() != nil {
		log.Fatalf("connect: %v", token.Error())
	}
	defer client.Disconnect(250)

	downTopic := mqttx.TopicDown(*deviceID)
	if token := client.Subscribe(downTopic, 1, nil); token.Wait() && token.Error() != nil {
		log.Fatalf("subscribe: %v", token.Error())
	}
	log.Printf("virtual light %s on %s (room=%s)", *deviceID, *broker, *room)

	publishUp(client, *deviceID, state, &mu)

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			publishUp(client, *deviceID, state, &mu)
		case <-sig:
			mu.Lock()
			state["online"] = false
			publishUp(client, *deviceID, state, &mu)
			mu.Unlock()
			return
		}
	}
}

func publishUp(client mqtt.Client, deviceID string, state map[string]any, mu *sync.Mutex) {
	mu.Lock()
	payload := map[string]any{}
	for k, v := range state {
		payload[k] = v
	}
	mu.Unlock()

	up := &mqttx.UpMessage{
		DeviceID: deviceID,
		Ts:       time.Now().UnixMilli(),
		Type:     "shadow",
		Payload:  payload,
	}
	b, _ := mqttx.EncodeUp(up)
	token := client.Publish(mqttx.TopicUp(deviceID), 1, false, b)
	token.Wait()
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
