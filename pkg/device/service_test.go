package device

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/cuteLittleDevil-ai/localmind-hub/pkg/mqttx"
	"github.com/cuteLittleDevil-ai/localmind-hub/pkg/store/sqlite"
	"github.com/redis/go-redis/v9"
)

func TestIdempotencyCachedResult(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	store, err := sqlite.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	mqtt := mqttx.NewClient(mqttx.Config{Broker: "tcp://127.0.0.1:1", ClientID: "test"})
	svc := NewService(store, rdb, mqtt)

	ctx := context.Background()
	_ = store.InsertDevice(&sqlite.Device{
		DeviceID: "d1", Type: "light", Name: "d1", Room: "living",
	})

	mr.Set("device:online:d1", "1")
	key := "idem-1"
	res := &ControlResult{CmdID: "c1", IdempotencyKey: key, OK: true, Code: 0, Message: "ok"}
	svc.saveIdempotency(ctx, "idempotency:"+key, res)

	ok, err := rdb.SetNX(ctx, "idempotency:"+key, `{"status":"processing"}`, time.Minute).Result()
	if !ok {
		raw, _ := rdb.Get(ctx, "idempotency:"+key).Bytes()
		var rec idempotencyRecord
		if json.Unmarshal(raw, &rec) != nil || rec.Result == nil {
			t.Fatal("expected cached result")
		}
		if !rec.Result.OK {
			t.Fatal(rec)
		}
	} else {
		t.Fatal("expected NX to fail")
	}
}

func TestMergeShadowFromUp(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	store, err := sqlite.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	mqtt := mqttx.NewClient(mqttx.Config{Broker: "tcp://127.0.0.1:1"})
	svc := NewService(store, rdb, mqtt)

	_ = store.InsertDevice(&sqlite.Device{DeviceID: "light-living-01", Type: "light", Name: "灯", Room: "living"})
	svc.handleUp("light-living-01", &mqttx.UpMessage{
		DeviceID: "light-living-01",
		Type:     "shadow",
		Payload:  map[string]any{"online": true, "brightness": float64(30), "room": "living"},
	})

	sh, err := svc.GetShadow(context.Background(), "light-living-01")
	if err != nil {
		t.Fatal(err)
	}
	if sh["brightness"].(float64) != 30 {
		t.Fatalf("shadow: %v", sh)
	}
	if !svc.IsOnline(context.Background(), "light-living-01") {
		t.Fatal("expected online")
	}
}
