package room

import (
	"math/rand"
	"sync/atomic"
	"testing"
	"time"

	"mahjong/server/internal/bot"
	"mahjong/server/internal/engine"
)

type fakeClient struct {
	ch     chan Message
	closed atomic.Bool
}

func newFakeClient() *fakeClient       { return &fakeClient{ch: make(chan Message, 10000)} }
func (f *fakeClient) Send(m Message)   { f.ch <- m }
func (f *fakeClient) Close()           { f.closed.Store(true) }
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
	r, s, err := m.Join("meja", "Ani", true, a)
	if err != nil || s != 0 {
		t.Fatalf("first join: seat %d err %v", s, err)
	}
	st := a.next(t, MsgRoomState).Data.(RoomState)
	if st.Host != 0 || st.YourSeat != 0 || st.Status != "waiting" || st.Seats[0].Kind != "human" {
		t.Errorf("room state after first join = %+v", st)
	}

	r2, s, err := m.Join("meja", "Budi", false, b)
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
	if _, _, err := m.Join("  ", "Ani", true, newFakeClient()); err != ErrBadName {
		t.Errorf("empty room name: %v", err)
	}
	if _, _, err := m.Join("meja", "nama yang sangat panjang sekali", true, newFakeClient()); err != ErrBadName {
		t.Errorf("long player name: %v", err)
	}
	for i := 0; i < engine.NumSeats; i++ {
		if _, _, err := m.Join("penuh", "P", i == 0, newFakeClient()); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := m.Join("penuh", "P5", false, newFakeClient()); err != ErrRoomFull {
		t.Errorf("fifth player: %v", err)
	}
}

func TestActBeforeStart(t *testing.T) {
	m := testManager()
	c := newFakeClient()
	r, s, _ := m.Join("meja", "Ani", true, c)
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
	r, me, err := m.Join("solo", "Ani", true, c)
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
	r, sa, _ := m.Join("meja", "Ani", true, a)
	_, sb, _ := m.Join("meja", "Budi", false, b)
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
	r, sa, _ := m.Join("meja", "Ani", true, a)
	_, sb, _ := m.Join("meja", "Budi", false, b)

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
	r, me, _ := m.Join("lambat", "Ani", true, c)
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

func TestRoomCodes(t *testing.T) {
	m := testManager()
	if _, _, err := m.Join("ABC12", "Ani", false, newFakeClient()); err != ErrRoomNotFound {
		t.Errorf("join unknown room: %v", err)
	}
	if m.Count() != 0 {
		t.Error("a failed join must not create a room")
	}
	r, _, err := m.Join("abc12", "Ani", true, newFakeClient())
	if err != nil {
		t.Fatal(err)
	}
	if r.Name() != "ABC12" {
		t.Errorf("room name %q, want it upper-cased", r.Name())
	}
	if _, _, err := m.Join("ABC12", "Budi", true, newFakeClient()); err != ErrRoomExists {
		t.Errorf("create an existing room: %v", err)
	}
	if r2, s, err := m.Join(" Abc12 ", "Budi", false, newFakeClient()); err != nil || r2 != r || s != 1 {
		t.Errorf("join by code in another case: seat %d err %v", s, err)
	}
}

func TestRemoveBot(t *testing.T) {
	m := testManager()
	a, b := newFakeClient(), newFakeClient()
	r, sa, _ := m.Join("meja", "Ani", true, a)
	_, sb, _ := m.Join("meja", "Budi", false, b)

	r.RemoveBot(sa, sb)
	if e := a.next(t, MsgError).Data.(*Error); e != ErrNotBot {
		t.Errorf("remove a human: %v", e)
	}
	r.AddBot(sa, 3)
	r.RemoveBot(sb, 3)
	if e := b.next(t, MsgError).Data.(*Error); e != ErrNotHost {
		t.Errorf("non-host remove_bot: %v", e)
	}
	r.RemoveBot(sa, 3)
	for {
		st := a.next(t, MsgRoomState).Data.(RoomState)
		if st.Seats[3].Kind == "empty" {
			break
		}
	}
}

// TestMultiplayerRounds: two humans and two bots play full rounds. Each human
// sees only their own hand and only the host may start the next round.
func TestMultiplayerRounds(t *testing.T) {
	m := testManager()
	clients := []*fakeClient{newFakeClient(), newFakeClient()}
	r, host, err := m.Join("teman", "Ani", true, clients[0])
	if err != nil {
		t.Fatal(err)
	}
	_, guest, err := m.Join("teman", "Budi", false, clients[1])
	if err != nil {
		t.Fatal(err)
	}
	seats := []engine.Seat{host, guest}
	r.AddBot(host, 2)
	r.AddBot(host, 3)
	r.Start(guest) // only the host may start
	if e := clients[1].next(t, MsgError).Data.(*Error); e != ErrNotHost {
		t.Fatalf("guest start_game: %v", e)
	}
	r.Start(host)

	type msgFrom struct {
		who int
		m   Message
	}
	all := make(chan msgFrom, 100)
	stop := make(chan struct{})
	defer close(stop)
	for i, c := range clients {
		go func(i int, c *fakeClient) {
			for {
				select {
				case m := <-c.ch:
					select {
					case all <- msgFrom{i, m}:
					case <-stop:
						return
					}
				case <-stop:
					return
				}
			}
		}(i, c)
	}

	players := []*bot.Bot{bot.New(1), bot.New(2)}
	const rounds = 3
	ended := [2]int{}
	deadline := time.After(30 * time.Second)
	for ended[0] < rounds || ended[1] < rounds {
		select {
		case <-deadline:
			t.Fatalf("rounds did not finish: %v", ended)
		case x := <-all:
			me := seats[x.who]
			switch x.m.Type {
			case MsgError:
				e := x.m.Data.(*Error)
				stale := e == ErrNotYourTurn || e == ErrIllegal || e == ErrNotInHand || e == ErrGameEnded
				if !stale && !(x.who == 1 && e == ErrNotHost) {
					t.Fatalf("player %d: unexpected error %v", x.who, e)
				}
			case MsgGameState:
				v := x.m.Data.(GameState).PlayerView
				if v.Seat != me || len(v.Hand) != v.HandCounts[me] {
					t.Fatalf("player %d got a wrong view: seat %d, %d tiles vs count %d", x.who, v.Seat, len(v.Hand), v.HandCounts[me])
				}
				if len(v.LegalActions) > 0 && v.Phase != engine.PhaseEnded {
					r.Act(me, players[x.who].Choose(v))
				}
			case MsgEvent:
				ev := x.m.Data.(EventData)
				if ev.Type == string(engine.EventDrew) && ev.Seat != me && ev.Tiles != nil {
					t.Fatalf("player %d saw seat %d's draw", x.who, ev.Seat)
				}
				if ev.Type == string(engine.EventGameEnd) {
					ended[x.who]++
					if x.who == 1 && ended[1] < rounds {
						r.NextRound(guest) // refused: not the host
					}
					if x.who == 0 && ended[0] < rounds {
						r.NextRound(host)
					}
				}
			}
		}
	}
}

// roomStateWith waits until c gets a room_state that satisfies ok.
func (f *fakeClient) roomStateWith(t *testing.T, ok func(RoomState) bool) RoomState {
	t.Helper()
	for {
		if st := f.next(t, MsgRoomState).Data.(RoomState); ok(st) {
			return st
		}
	}
}

// inspect runs f on the room goroutine and waits for it.
func inspect[T any](r *Room, f func() T) T {
	ch := make(chan T, 1)
	r.post(func() { ch <- f() })
	return <-ch
}

func TestReconnectDuringGame(t *testing.T) {
	m := testManager()
	a, b := newFakeClient(), newFakeClient()
	r, sa, _ := m.Join("meja", "Ani", true, a)
	_, sb, _ := m.Join("meja", "Budi", false, b)
	token := b.next(t, MsgRoomState).Data.(RoomState).Token
	if len(token) != 32 {
		t.Fatalf("token %q", token)
	}
	r.AddBot(sa, 2)
	r.AddBot(sa, 3)
	r.Start(sa)

	r.Disconnect(sb, b)
	st := a.roomStateWith(t, func(st RoomState) bool { return st.Status == "playing" && !st.Seats[sb].Connected })
	if st.Seats[sb].Kind != "human" {
		t.Fatalf("away seat as others see it: %+v", st.Seats[sb])
	}
	if st.Token == token || st.Token == "" {
		t.Fatal("each player must get only their own token")
	}

	if _, _, err := m.Rejoin("meja", "salah", newFakeClient()); err != ErrBadToken {
		t.Errorf("wrong token: %v", err)
	}
	if _, _, err := m.Rejoin("tidakada", token, newFakeClient()); err != ErrRoomNotFound {
		t.Errorf("unknown room: %v", err)
	}
	// Joining by name is not possible once the game runs; only rejoin is.
	if _, _, err := m.Join("meja", "Budi", false, newFakeClient()); err != ErrGameStarted {
		t.Errorf("join a running game: %v", err)
	}

	b2 := newFakeClient()
	r2, s, err := m.Rejoin("MEJA", token, b2)
	if err != nil || r2 != r || s != sb {
		t.Fatalf("rejoin: seat %d err %v", s, err)
	}
	if st := b2.next(t, MsgRoomState).Data.(RoomState); !st.Seats[sb].Connected || st.YourSeat != sb || st.Token != token {
		t.Errorf("room state after rejoin: %+v", st)
	}
	if v := b2.next(t, MsgGameState).Data.(GameState); v.Seat != sb || len(v.Hand) == 0 {
		t.Errorf("game state after rejoin: seat %d, %d tiles", v.Seat, len(v.Hand))
	}
	// The old connection dropping late must not mark the player away again.
	r.Disconnect(sb, b)
	if inspect(r, func() bool { return r.seats[sb].connected() }) != true {
		t.Error("stale disconnect kicked the rejoined player")
	}
}

func TestRejoinReplacesOldConnection(t *testing.T) {
	m := testManager()
	old := newFakeClient()
	r, s, _ := m.Join("meja", "Ani", true, old)
	token := old.next(t, MsgRoomState).Data.(RoomState).Token

	fresh := newFakeClient()
	if _, got, err := m.Rejoin("meja", token, fresh); err != nil || got != s {
		t.Fatalf("rejoin while connected: seat %d err %v", got, err)
	}
	if e := old.next(t, MsgError).Data.(*Error); e != ErrSessionMoved {
		t.Errorf("old connection told %v", e)
	}
	if !old.closed.Load() {
		t.Error("old connection not closed")
	}
	r.Disconnect(s, old)
	if !inspect(r, func() bool { return r.seats[s].client == Client(fresh) }) {
		t.Error("the new connection lost its seat")
	}
}

// TestFreezeWhileEveryoneAway: with nobody connected the round must not play
// itself out; it continues when someone comes back.
func TestFreezeWhileEveryoneAway(t *testing.T) {
	m := NewManager(Options{BotDelay: func(*rand.Rand) time.Duration { return time.Millisecond }, Seed: 9, TimeUnit: time.Millisecond})
	c := newFakeClient()
	r, me, _ := m.Join("solo", "Ani", true, c)
	token := c.next(t, MsgRoomState).Data.(RoomState).Token
	r.UpdateSettings(me, Settings{TurnSeconds: 5, ClaimSeconds: 3}) // 5ms / 3ms turns
	for s := engine.Seat(1); s < engine.NumSeats; s++ {
		r.AddBot(me, s)
	}
	r.Start(me)
	c.next(t, MsgGameState)

	r.Disconnect(me, c)
	before := inspect(r, func() int { return r.game.WallLeft() })
	time.Sleep(100 * time.Millisecond) // many turns' worth of time
	if after := inspect(r, func() int { return r.game.WallLeft() }); after != before {
		t.Fatalf("game moved while nobody was connected: wall %d -> %d", before, after)
	}

	c2 := newFakeClient()
	if _, _, err := m.Rejoin("solo", token, c2); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool {
		return inspect(r, func() int { return r.game.WallLeft() }) < before || inspect(r, r.game.Ended)
	})
}

func TestIdleRoomCloses(t *testing.T) {
	m := NewManager(Options{BotDelay: noDelay, Seed: 1, TimeUnit: 100 * time.Microsecond}) // 5 min -> 30ms
	c := newFakeClient()
	r, me, _ := m.Join("meja", "Ani", true, c)
	r.AddBot(me, 1)
	r.AddBot(me, 2)
	r.AddBot(me, 3)
	r.Start(me)
	r.Disconnect(me, c)
	if m.Count() != 1 {
		t.Fatal("room closed at once; it should wait for the player")
	}
	waitFor(t, func() bool { return m.Count() == 0 })
}

func TestWaitingRoomGrace(t *testing.T) {
	m := NewManager(Options{BotDelay: noDelay, Seed: 1, TimeUnit: time.Millisecond}) // 60s -> 60ms
	a, b := newFakeClient(), newFakeClient()
	r, sa, _ := m.Join("meja", "Ani", true, a)
	_, sb, _ := m.Join("meja", "Budi", false, b)

	// The host's connection drops: Budi becomes host, Ani's seat is kept for a while.
	r.Disconnect(sa, a)
	st := b.roomStateWith(t, func(st RoomState) bool { return st.Host == sb })
	if st.Seats[sa].Kind != "human" || st.Seats[sa].Connected {
		t.Errorf("away host's seat right after the drop: %+v", st.Seats[sa])
	}
	b.roomStateWith(t, func(st RoomState) bool { return st.Seats[sa].Kind == "empty" })
}
