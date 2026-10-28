package mqttx

import (
	"fmt"
	"strings"
)

const TopicPrefix = "localmind"

func TopicUp(deviceID string) string {
	return fmt.Sprintf("%s/%s/up", TopicPrefix, deviceID)
}

func TopicDown(deviceID string) string {
	return fmt.Sprintf("%s/%s/down", TopicPrefix, deviceID)
}

func TopicAck(deviceID string) string {
	return fmt.Sprintf("%s/%s/ack", TopicPrefix, deviceID)
}

const subUp = "localmind/+/up"
const subAck = "localmind/+/ack"

// ParseDeviceFromTopic extracts deviceId from localmind/{deviceId}/up|down|ack.
func ParseDeviceFromTopic(topic string) (deviceID string, kind string, ok bool) {
	parts := strings.Split(topic, "/")
	if len(parts) != 3 || parts[0] != TopicPrefix {
		return "", "", false
	}
	return parts[1], parts[2], true
}
