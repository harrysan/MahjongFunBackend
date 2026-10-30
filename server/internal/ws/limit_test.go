package ws

import (
	"context"
	"encoding/json"
	"math/rand"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"mahjong/server/internal/room"
)

func TestLimiter(t *testing.T) {
	l := newLimiter(10, 3)
	now := time.Unix(1000, 0)
	for i := 0; i < 3; i++ {
		if !l.allow(now) {
			t.Fatalf("message %d of the burst refused", i+1)
		}
	}
	if l.allow(now) {
		t.Error("message over the burst allowed")
	}
	if !l.allow(now.Add(100 * time.Millisecond)) {
		t.Error("one token should refill after 1/rate seconds")
	}
	if l.allow(now.Add(100 * time.Millisecond)) {
		t.Error("only one token refilled")
	}
	// A long pause refills to the burst, not beyond.
	now = now.Add(time.Hour)
	for i := 0; i < 3; i++ {
		if !l.allow(now) {
			t.Fatalf("after a pause: message %d refused", i+1)
		}
	}
	if l.allow(now) {
		t.Error("refill went over the burst")
	}
}

func dial(t *testing.T, ctx context.Context, url string) *websocket.Conn {
	t.Helper()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(url, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// TestFloodIsCut sends far more than a person could: the client is told to
// slow down once, then disconnected.
func TestFloodIsCut(t *testing.T) {
	rooms := room.NewManager(room.Options{BotDelay: func(*rand.Rand) time.Duration { return time.Hour }, Seed: 1})
	srv := httptest.NewServer(&Handler{Rooms: rooms})
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c := dial(t, ctx, srv.URL)
	defer c.CloseNow()

	go func() {
		for i := 0; i < msgBurst+maxDropped+50; i++ {
			if c.Write(ctx, websocket.MessageText, []byte(`{"type":"start_game"}`)) != nil {
				return
			}
		}
	}()
	limited := 0
	for {
		var m msg
		err := wsjson.Read(ctx, c, &m)
		if err != nil {
			if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
				t.Fatalf("connection ended with %v, want a policy-violation close", err)
			}
			break
		}
		var e room.Error
		_ = json.Unmarshal(m.Data, &e)
		if e.Code == "rate_limited" {
			limited++
		}
	}
	if limited != 1 {
		t.Errorf("got %d rate_limited errors, want exactly 1", limited)
	}
}

// TestJunkDoesNotHurtServer sends malformed and oversized messages; the
// server answers or drops that connection, and keeps serving others.
func TestJunkDoesNotHurtServer(t *testing.T) {
	rooms := room.NewManager(room.Options{BotDelay: func(*rand.Rand) time.Duration { return time.Hour }, Seed: 1})
	srv := httptest.NewServer(&Handler{Rooms: rooms})
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	junk := []string{
		``, `null`, `[]`, `"text"`, `{"type":1}`, `{"type":"discard","data":"x"}`,
		`{"type":"discard","data":{"tile_id":-5}}`, `{"type":"claim","data":{"action":"chi","tiles":[999,1]}}`,
		`{"type":"join_room","data":{"room":"` + strings.Repeat("A", 100) + `","name":"x","create":true}}`,
		`{"type":"update_settings","data":{"turn_seconds":-1}}`, `{"type":"nonsense"}`, "\xff\xfe",
	}
	c := dial(t, ctx, srv.URL)
	defer c.CloseNow()
	for _, j := range junk {
		if err := c.Write(ctx, websocket.MessageText, []byte(j)); err != nil {
			t.Fatal(err)
		}
		var m msg
		if err := wsjson.Read(ctx, c, &m); err != nil || m.Type != "error" {
			t.Fatalf("junk %q: got %+v, %v; want an error reply", j, m, err)
		}
	}
	if err := c.Write(ctx, websocket.MessageBinary, []byte{0, 1, 2}); err != nil {
		t.Fatal(err)
	}
	if err := c.Write(ctx, websocket.MessageText, []byte(strings.Repeat("x", maxReadBytes+1))); err != nil {
		t.Fatal(err)
	}
	for {
		var m msg
		if err := wsjson.Read(ctx, c, &m); err != nil {
			break // the oversized message closed the connection
		}
	}

	// Another player is not affected.
	c2 := dial(t, ctx, srv.URL)
	defer c2.CloseNow()
	if err := wsjson.Write(ctx, c2, map[string]any{"type": "join_room", "data": map[string]any{"room": "ok", "name": "Ani", "create": true}}); err != nil {
		t.Fatal(err)
	}
	var m msg
	if err := wsjson.Read(ctx, c2, &m); err != nil || m.Type != "room_state" {
		t.Fatalf("after junk: got %+v, %v; want room_state", m, err)
	}
	if rooms.Count() != 1 {
		t.Errorf("%d rooms open, want 1 (junk must not create rooms)", rooms.Count())
	}
}
