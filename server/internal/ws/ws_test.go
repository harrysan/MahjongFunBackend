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

	"mahjong/server/internal/bot"
	"mahjong/server/internal/engine"
	"mahjong/server/internal/room"
)

func TestParseAction(t *testing.T) {
	cases := []struct {
		typ, data string
		want      engine.Action
		ok        bool
	}{
		{"discard", `{"tile_id": 12}`, engine.Action{Type: engine.ActionDiscard, TileID: 12}, true},
		{"discard", `{}`, engine.Action{}, false},
		{"claim", `{"action": "pass"}`, engine.Action{Type: engine.ActionPass}, true},
		{"claim", `{"action": "pung"}`, engine.Action{Type: engine.ActionPung}, true},
		{"claim", `{"action": "hu"}`, engine.Action{Type: engine.ActionHu}, true},
		{"claim", `{"action": "chi", "tiles": [4, 9]}`, engine.Action{Type: engine.ActionChi, TileID: 4, TileID2: 9}, true},
		{"claim", `{"action": "chi", "tiles": [4]}`, engine.Action{}, false},
		{"claim", `{"action": "discard"}`, engine.Action{}, false},
		{"claim", `{"action": "ron"}`, engine.Action{}, false},
		{"self_action", `{"action": "kong", "tile_id": 40}`, engine.Action{Type: engine.ActionKong, TileID: 40}, true},
		{"self_action", `{"action": "kong"}`, engine.Action{}, false},
		{"self_action", `{"action": "hu"}`, engine.Action{Type: engine.ActionHu}, true},
		{"self_action", `{"action": "pung"}`, engine.Action{}, false},
	}
	for _, tc := range cases {
		got, ok := parseAction(envelope{Type: tc.typ, Data: json.RawMessage(tc.data)})
		if ok != tc.ok || got != tc.want {
			t.Errorf("%s %s = %+v, %v; want %+v, %v", tc.typ, tc.data, got, ok, tc.want, tc.ok)
		}
	}
}

// msg is a server message as the browser sees it.
type msg struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// TestSoloOverWebSocket plays one solo round through a real WebSocket connection.
func TestSoloOverWebSocket(t *testing.T) {
	rooms := room.NewManager(room.Options{BotDelay: func(*rand.Rand) time.Duration { return 0 }, Seed: 3})
	// Bots answer instantly, so this player sends far faster than a human could.
	srv := httptest.NewServer(&Handler{Rooms: rooms, MsgRate: 1e6, MsgBurst: 1e6})
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()

	send := func(typ string, data any) {
		if err := wsjson.Write(ctx, c, map[string]any{"type": typ, "data": data}); err != nil {
			t.Fatal(err)
		}
	}

	// Garbage is answered with an error, the connection stays open.
	if err := c.Write(ctx, websocket.MessageText, []byte("{not json")); err != nil {
		t.Fatal(err)
	}
	send("start_game", nil)
	send("join_room", map[string]any{"room": "nope", "name": "Ani"}) // does not exist: error
	send("join_room", map[string]any{"room": "solo", "name": "Ani", "create": true})
	for s := 1; s < engine.NumSeats; s++ {
		send("add_bot", map[string]int{"seat": s})
	}
	send("start_game", nil)

	player := bot.New(1)
	errorsSeen := 0
	for {
		var m msg
		if err := wsjson.Read(ctx, c, &m); err != nil {
			t.Fatal(err)
		}
		switch m.Type {
		case "error":
			errorsSeen++
		case "game_state":
			var v struct {
				Phase        string `json:"phase"`
				Hand         []engine.Tile
				LegalActions []struct {
					Type    string `json:"type"`
					TileID  uint8  `json:"tile_id"`
					TileID2 uint8  `json:"tile_id2"`
				} `json:"legal_actions"`
			}
			if err := json.Unmarshal(m.Data, &v); err != nil {
				t.Fatal(err)
			}
			if len(v.LegalActions) == 0 || v.Phase == "Ended" {
				continue
			}
			// Let the bot pick, then send it the way the UI would.
			view := engine.PlayerView{Hand: v.Hand, Phase: engine.PhaseDiscarding}
			if v.Phase == "ClaimWindow" {
				view.Phase = engine.PhaseClaim
			}
			for _, la := range v.LegalActions {
				typ, _ := engine.ParseActionType(la.Type)
				view.LegalActions = append(view.LegalActions, engine.Action{Type: typ, TileID: la.TileID, TileID2: la.TileID2})
			}
			a := player.Choose(view)
			switch {
			case a.Type == engine.ActionDiscard:
				send("discard", map[string]any{"tile_id": a.TileID})
			case view.Phase == engine.PhaseClaim:
				send("claim", map[string]any{"action": a.Type.String(), "tiles": []uint8{a.TileID, a.TileID2}})
			default:
				send("self_action", map[string]any{"action": a.Type.String(), "tile_id": a.TileID})
			}
		case "event":
			var e struct {
				Type   string         `json:"type"`
				Result *engine.Result `json:"result"`
			}
			if err := json.Unmarshal(m.Data, &e); err != nil {
				t.Fatal(err)
			}
			if e.Type == "game_end" {
				if e.Result == nil {
					t.Fatal("game_end without result")
				}
				if errorsSeen < 3 {
					t.Errorf("expected errors for garbage, early start_game and unknown room, saw %d", errorsSeen)
				}
				return
			}
		}
	}
}

// TestRejoinOverWebSocket: a player whose connection drops gets their seat
// back on a new connection with the token from room_state.
func TestRejoinOverWebSocket(t *testing.T) {
	rooms := room.NewManager(room.Options{BotDelay: func(*rand.Rand) time.Duration { return time.Hour }, Seed: 3})
	srv := httptest.NewServer(&Handler{Rooms: rooms})
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")

	dial := func() *websocket.Conn {
		c, _, err := websocket.Dial(ctx, url, nil)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	send := func(c *websocket.Conn, typ string, data any) {
		if err := wsjson.Write(ctx, c, map[string]any{"type": typ, "data": data}); err != nil {
			t.Fatal(err)
		}
	}
	// next reads until a message of type typ arrives.
	next := func(c *websocket.Conn, typ string) json.RawMessage {
		for {
			var m msg
			if err := wsjson.Read(ctx, c, &m); err != nil {
				t.Fatal(err)
			}
			if m.Type == typ {
				return m.Data
			}
		}
	}
	type roomState struct {
		Room     string `json:"room"`
		Status   string `json:"status"`
		YourSeat int    `json:"your_seat"`
		Token    string `json:"token"`
		Seats    []struct {
			Kind      string `json:"kind"`
			Connected bool   `json:"connected"`
		} `json:"seats"`
	}

	c1 := dial()
	send(c1, "join_room", map[string]any{"room": "kumpul", "name": "Ani", "create": true})
	var st roomState
	json.Unmarshal(next(c1, "room_state"), &st)
	for s := 1; s < engine.NumSeats; s++ {
		send(c1, "add_bot", map[string]int{"seat": s})
	}
	send(c1, "start_game", nil)
	next(c1, "game_state")
	c1.Close(websocket.StatusGoingAway, "refresh")

	c2 := dial()
	defer c2.CloseNow()
	send(c2, "rejoin_room", map[string]string{"room": st.Room, "token": "salah"})
	var e room.Error
	json.Unmarshal(next(c2, "error"), &e)
	if e.Code != "bad_token" {
		t.Errorf("rejoin with a wrong token: %+v", e)
	}
	send(c2, "rejoin_room", map[string]string{"room": st.Room, "token": st.Token})
	var st2 roomState
	for st2.Status != "playing" || !st2.Seats[0].Connected {
		json.Unmarshal(next(c2, "room_state"), &st2)
	}
	if st2.YourSeat != st.YourSeat {
		t.Errorf("rejoined at seat %d, want %d", st2.YourSeat, st.YourSeat)
	}
	next(c2, "game_state")

	// leave_room gives the seat up for good: the token stops working.
	send(c2, "leave_room", nil)
	c3 := dial()
	defer c3.CloseNow()
	time.Sleep(50 * time.Millisecond)
	send(c3, "rejoin_room", map[string]string{"room": st.Room, "token": st.Token})
	json.Unmarshal(next(c3, "error"), &e)
	if e.Code != "room_not_found" && e.Code != "bad_token" {
		t.Errorf("rejoin after leaving: %+v", e)
	}
}
