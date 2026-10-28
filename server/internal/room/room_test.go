package room

import (
	"math/rand"
	"testing"
	"time"

	"mahjong/server/internal/bot"
	"mahjong/server/internal/engine"
)

type fakeClient struct{ ch chan Message }

func newFakeClient() *fakeClient       { return &fakeClient{ch: make(chan Message, 10000)} }
func (f *fakeClient) Send(m Message)   { f.ch <- m }
func noDelay(*rand.Rand) time.Duration { return 0 }

func testManager() *Manager { return NewManager(Options{BotDelay: noDelay, Seed: 42}) }

// next returns the next message whose type is typ, failing after a timeout.
func (f *fakeClient) next(t *testing.T, typ string) Message {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case m := <-f.ch:
			if m.Type == typ {
				return m
			}
		case <-timeout:
			t.Fatalf("timed out waiting for %q", typ)
		}
	}
}

func TestJoinAndHost(t *testing.T) {
	m := testManager()
	a, b := newFakeClient(), newFakeClient()
	r, s, err := m.Join("meja", "Ani", a)
	if err != nil || s != 0 {
		t.Fatalf("first join: seat %d err %v", s, err)
	}
	st := a.next(t, MsgRoomState).Data.(RoomState)
	if st.Host != 0 || st.YourSeat != 0 || st.Status != "waiting" || st.Seats[0].Kind != "human" {
		t.Errorf("room state after first join = %+v", st)
	}

	r2, s, err := m.Join("meja", "Budi", b)
	if err != nil || s != 1 || r2 != r {
		t.Fatalf("second join: seat %d err %v same room %v", s, err, r2 == r)
	}
	st = b.next(t, MsgRoomState).Data.(RoomState)
	if st.Host != 0 || st.YourSeat != 1 {
		t.Errorf("second player sees %+v", st)
	}

	// Only the host may add bots or start.
	r.AddBot(1, 2)
	if e := b.next(t, MsgError).Data.(*Error); e != ErrNotHost {
		t.Errorf("non-host add_bot: %v", e)
	}
	r.Start(0)
	if e := a.next(t, MsgError).Data.(*Error); e != ErrRoomNotFull {
		t.Errorf("start with empty seats: %v", e)
	}
	r.AddBot(0, 1)
	if e := a.next(t, MsgError).Data.(*Error); e != ErrSeatTaken {
		t.Errorf("bot on a taken seat: %v", e)
	}
	r.AddBot(0, 7)
	if e := a.next(t, MsgError).Data.(*Error); e != ErrBadSeat {
		t.Errorf("bot on seat 7: %v", e)
	}

	// The host leaves: the other human becomes host.
	r.Leave(0)
	st = b.next(t, MsgRoomState).Data.(RoomState)
	if st.Host != 1 || st.Seats[0].Kind != "empty" {
		t.Errorf("after host left: %+v", st)
	}
	// The last human leaves: the room closes.
	r.Leave(1)
	waitFor(t, func() bool { return m.Count() == 0 })
}

func TestJoinValidation(t *testing.T) {
	m := testManager()
	if _, _, err := m.Join("  ", "Ani", newFakeClient()); err != ErrBadName {
		t.Errorf("empty room name: %v", err)
	}
	if _, _, err := m.Join("meja", "nama yang sangat panjang sekali", newFakeClient()); err != ErrBadName {
		t.Errorf("long player name: %v", err)
	}
	for i := 0; i < engine.NumSeats; i++ {
		if _, _, err := m.Join("penuh", "P", newFakeClient()); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := m.Join("penuh", "P5", newFakeClient()); err != ErrRoomFull {
		t.Errorf("fifth player: %v", err)
	}
}

func TestActBeforeStart(t *testing.T) {
	m := testManager()
	c := newFakeClient()
	r, s, _ := m.Join("meja", "Ani", c)
	r.Act(s, engine.Action{Type: engine.ActionPass})
	if e := c.next(t, MsgError).Data.(*Error); e != ErrNoGame {
		t.Errorf("act before start: %v", e)
	}
	r.NextRound(s)
	if e := c.next(t, MsgError).Data.(*Error); e != ErrNoGame {
		t.Errorf("next_round before start: %v", e)
	}
}

// TestSoloRounds plays several full solo rounds: one human (driven by the bot
// logic, the way a person clicking buttons would) against three bots.
func TestSoloRounds(t *testing.T) {
	m := testManager()
	c := newFakeClient()
	r, me, err := m.Join("solo", "Ani", c)
	if err != nil {
		t.Fatal(err)
	}
	for s := engine.Seat(1); s < engine.NumSeats; s++ {
		r.AddBot(me, s)
	}
	r.Start(me)

	player := bot.New(7)
	const rounds = 5
	deadline := time.After(30 * time.Second)
	for round := 0; round < rounds; {
		select {
		case <-deadline:
			t.Fatalf("round %d did not finish", round)
		case msg := <-c.ch:
			switch msg.Type {
			case MsgError:
				// Acting on a stale snapshot (one sent before our previous action
				// was handled) is refused; a fresh game_state always follows.
				if e := msg.Data.(*Error); e != ErrNotYourTurn && e != ErrIllegal && e != ErrNotInHand {
					t.Fatalf("unexpected error: %v", e)
				}
			case MsgGameState:
				v := msg.Data.(GameState).PlayerView
				if v.Seat != me {
					t.Fatalf("got the view of seat %d", v.Seat)
				}
				if len(v.Hand) != v.HandCounts[me] {
					t.Fatalf("hand has %d tiles, hand_counts says %d", len(v.Hand), v.HandCounts[me])
				}
				if len(v.LegalActions) > 0 && v.Phase != engine.PhaseEnded {
					r.Act(me, player.Choose(v))
				}
			case MsgEvent:
				ev := msg.Data.(EventData)
				if ev.Type == string(engine.EventDrew) && ev.Seat != me && ev.Tiles != nil {
					t.Fatalf("saw another player's drawn tile: %+v", ev)
				}
				if ev.Type == string(engine.EventGameEnd) {
					if ev.Result == nil {
						t.Fatal("game_end without result")
					}
					round++
					if round < rounds {
						r.NextRound(me)
					}
				}
				if ev.Type == EventDiceRolled {
					if ev.Dice == nil || ev.NewDealer == nil {
						t.Fatalf("dice_rolled without dice: %+v", ev)
					}
					if want := engine.NextDealer(ev.Seat, *ev.Dice); want != *ev.NewDealer {
						t.Errorf("new dealer %d, want %d", *ev.NewDealer, want)
					}
				}
			}
		}
	}
	r.Leave(me)
	waitFor(t, func() bool { return m.Count() == 0 })
}

func TestLeaveDuringGameBotTakesOver(t *testing.T) {
	m := testManager()
	a, b := newFakeClient(), newFakeClient()
	r, sa, _ := m.Join("meja", "Ani", a)
	_, sb, _ := m.Join("meja", "Budi", b)
	r.AddBot(sa, 2)
	r.AddBot(sa, 3)
	r.Start(sa)

	// Budi leaves right away; a bot plays his seat. Ani always picks her last
	// legal action (pass, or her last tile).
	r.Leave(sb)
	tookOver := false
	deadline := time.After(30 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("round did not finish")
		case msg := <-a.ch:
			switch msg.Type {
			case MsgRoomState:
				tookOver = tookOver || msg.Data.(RoomState).Seats[sb].Kind == "bot"
			case MsgGameState:
				v := msg.Data.(GameState).PlayerView
				if len(v.LegalActions) > 0 {
					r.Act(sa, v.LegalActions[len(v.LegalActions)-1])
				}
			case MsgEvent:
				if msg.Data.(EventData).Type == string(engine.EventGameEnd) {
					if !tookOver {
						t.Error("seat was never shown as a bot")
					}
					return
				}
			}
		}
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not reached")
}

func TestSettingsValidation(t *testing.T) {
	ok := []Settings{
		DefaultSettings(),
		{BotDelayMinMs: 0, BotDelayMaxMs: 0, TurnSeconds: 0, ClaimSeconds: 0},
		{BotDelayMinMs: 2000, BotDelayMaxMs: 10000, TurnSeconds: 120, ClaimSeconds: 3},
	}
	bad := []Settings{
		{BotDelayMinMs: 1000, BotDelayMaxMs: 500, TurnSeconds: 30, ClaimSeconds: 10},
		{BotDelayMinMs: -1, BotDelayMaxMs: 500, TurnSeconds: 30, ClaimSeconds: 10},
		{BotDelayMinMs: 0, BotDelayMaxMs: 20000, TurnSeconds: 30, ClaimSeconds: 10},
		{BotDelayMinMs: 0, BotDelayMaxMs: 500, TurnSeconds: 2, ClaimSeconds: 10},
		{BotDelayMinMs: 0, BotDelayMaxMs: 500, TurnSeconds: 30, ClaimSeconds: 99},
	}
	for _, s := range ok {
		if !s.Valid() {
			t.Errorf("%+v should be valid", s)
		}
	}
	for _, s := range bad {
		if s.Valid() {
			t.Errorf("%+v should be invalid", s)
		}
	}
}

func TestUpdateSettings(t *testing.T) {
	m := testManager()
	a, b := newFakeClient(), newFakeClient()
	r, sa, _ := m.Join("meja", "Ani", a)
	_, sb, _ := m.Join("meja", "Budi", b)

	want := Settings{BotDelayMinMs: 100, BotDelayMaxMs: 300, TurnSeconds: 15, ClaimSeconds: 5}
	r.UpdateSettings(sb, want)
	if e := b.next(t, MsgError).Data.(*Error); e != ErrNotHost {
		t.Errorf("non-host update: %v", e)
	}
	r.UpdateSettings(sa, Settings{TurnSeconds: 1})
	if e := a.next(t, MsgError).Data.(*Error); e != ErrBadSettings {
		t.Errorf("bad settings: %v", e)
	}
	r.UpdateSettings(sa, want)
	for {
		st := b.next(t, MsgRoomState).Data.(RoomState)
		if st.Settings == want {
			break
		}
	}
}

// TestTimeouts: a human who never acts is played for by the clock, so the
// round still finishes (spec 4.6).
func TestTimeouts(t *testing.T) {
	m := NewManager(Options{BotDelay: noDelay, Seed: 5, TimeUnit: 5 * time.Millisecond})
	c := newFakeClient()
	r, me, _ := m.Join("lambat", "Ani", c)
	r.UpdateSettings(me, Settings{TurnSeconds: 5, ClaimSeconds: 3})
	for s := engine.Seat(1); s < engine.NumSeats; s++ {
		r.AddBot(me, s)
	}
	r.Start(me)

	deadline := time.After(30 * time.Second)
	sawClock, sawMyDiscard := false, false
	for {
		select {
		case <-deadline:
			t.Fatal("round did not finish on timeouts")
		case msg := <-c.ch:
			switch msg.Type {
			case MsgGameState:
				if gs := msg.Data.(GameState); gs.Phase != engine.PhaseEnded && gs.TimeLeftMs > 0 {
					sawClock = true
				}
			case MsgEvent:
				ev := msg.Data.(EventData)
				if ev.Type == string(engine.EventDiscarded) && ev.Seat == me {
					sawMyDiscard = true
				}
				if ev.Type == string(engine.EventGameEnd) {
					if !sawClock {
						t.Error("game_state never carried time_left_ms")
					}
					// Ani is the first dealer, so she must discard before anything else happens.
					if !sawMyDiscard {
						t.Error("the clock never discarded for the idle player")
					}
					return
				}
			}
		}
	}
}

func TestAutoDiscard(t *testing.T) {
	d := engine.Tile{ID: 9, Kind: 2}
	acts := []engine.Action{
		{Type: engine.ActionKong, TileID: 1},
		{Type: engine.ActionDiscard, TileID: 9},
		{Type: engine.ActionDiscard, TileID: 30},
	}
	if a := autoDiscard(engine.PlayerView{Drawn: &d, LegalActions: acts}); a.TileID != 9 || a.Type != engine.ActionDiscard {
		t.Errorf("with a drawn tile: %+v", a)
	}
	if a := autoDiscard(engine.PlayerView{LegalActions: acts}); a.TileID != 30 {
		t.Errorf("after a claim (no drawn tile): %+v", a)
	}
}
