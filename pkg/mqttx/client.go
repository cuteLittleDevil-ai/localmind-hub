package mqttx

import (
	"fmt"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// UpHandler processes uplink messages.
type UpHandler func(deviceID string, msg *UpMessage)

// AckHandler processes ack messages.
type AckHandler func(deviceID string, msg *AckMessage)

// Client wraps EMQX connectivity for localmind topics.
type Client struct {
	broker   string
	clientID string
	mu       sync.Mutex
	c        mqtt.Client
	onUp     UpHandler
	onAck    AckHandler
}

// Config for MQTT client.
type Config struct {
	Broker   string
	ClientID string
}

// NewClient creates a disconnected client; call Connect to subscribe.
func NewClient(cfg Config) *Client {
	if cfg.ClientID == "" {
		cfg.ClientID = fmt.Sprintf("localmind-hub-%d", time.Now().UnixNano())
	}
	return &Client{broker: cfg.Broker, clientID: cfg.ClientID}
}

// SetHandlers registers uplink callbacks (must be set before Connect).
func (c *Client) SetHandlers(onUp UpHandler, onAck AckHandler) {
	c.onUp = onUp
	c.onAck = onAck
}

// Connect dials broker and subscribes to localmind/+/up and localmind/+/ack.
// 连接 EMQX 后订阅上行与 ack，解码 JSON 后回调 device 域（影子/控制 pending）。
func (c *Client) Connect() error {
	opts := mqtt.NewClientOptions().
		AddBroker(c.broker).
		SetClientID(c.clientID).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetOrderMatters(false)

	opts.SetDefaultPublishHandler(func(_ mqtt.Client, msg mqtt.Message) {
		topic := msg.Topic()
		deviceID, kind, ok := ParseDeviceFromTopic(topic)
		if !ok {
			return
		}
		switch kind {
		case "up":
			if c.onUp == nil {
				return
			}
			up, err := DecodeUp(msg.Payload())
			if err != nil {
				return
			}
			if up.DeviceID == "" {
				up.DeviceID = deviceID
			}
			c.onUp(deviceID, up)
		case "ack":
			if c.onAck == nil {
				return
			}
			ack, err := DecodeAck(msg.Payload())
			if err != nil {
				return
			}
			c.onAck(deviceID, ack)
		}
	})

	c.mu.Lock()
	defer c.mu.Unlock()
	c.c = mqtt.NewClient(opts)
	token := c.c.Connect()
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}

	if token := c.c.Subscribe(subUp, 1, nil); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	if token := c.c.Subscribe(subAck, 1, nil); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

// PublishDown sends control to localmind/{deviceId}/down.
func (c *Client) PublishDown(deviceID string, msg *DownMessage) error {
	c.mu.Lock()
	client := c.c
	c.mu.Unlock()
	if client == nil || !client.IsConnected() {
		return fmt.Errorf("mqtt not connected")
	}
	payload, err := EncodeDown(msg)
	if err != nil {
		return err
	}
	token := client.Publish(TopicDown(deviceID), 1, false, payload)
	token.Wait()
	return token.Error()
}

// Disconnect closes the MQTT session.
func (c *Client) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.c != nil && c.c.IsConnected() {
		c.c.Disconnect(250)
	}
}

// IsConnected reports broker session state.
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.c != nil && c.c.IsConnected()
}
