// Package room runs tables: seats, the round lifecycle and bot turns.
//
// Each Room owns one goroutine. Every state change happens on that goroutine,
// so the engine (which is not safe for concurrent use) needs no locks. The
// exported methods only queue work for it and report errors to the client.
package room

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	mrand "math/rand"
	"time"

	"mahjong/server/internal/bot"
	"mahjong/server/internal/engine"
)

type seatKind uint8

const (
	seatEmpty seatKind = iota
	seatHuman
	seatBot
)

var seatKindNames = [...]string{seatEmpty: "empty", seatHuman: "human", seatBot: "bot"}

type seat struct {
	kind   seatKind
	name   string
	token  string   // human seats: secret that lets the player reconnect
	client Client   // human seats: nil while the player is disconnected
	bot    *bot.Bot // plays bot seats, and human seats while their player is away
}

func (s *seat) connected() bool { return s.kind == seatHuman && s.client != nil }

// botPlays reports whether the computer makes this seat's moves.
func (s *seat) botPlays() bool { return s.kind == seatBot || (s.kind == seatHuman && s.client == nil) }

// Options tune a room; the zero value means the defaults from config.go.
type Options struct {
	// BotDelay returns how long a bot waits before acting, overriding the
	// room settings (used by tests). nil = random within the settings.
	BotDelay func(rng *mrand.Rand) time.Duration
	// Seed seeds the room's dice, shuffles and bots. 0 = time based.
	Seed int64
	// TimeUnit is the length of one "second" for every room clock (turns,
	// claims, reconnect grace, idle close). 0 = time.Second; tests shrink it.
	TimeUnit time.Duration
	// MaxRooms caps how many rooms may be open at once. 0 = MaxRooms.
	MaxRooms int
	// Logger receives room lifecycle logs. nil = slog.Default().
	Logger *slog.Logger
}

func (o Options) logger() *slog.Logger {
	if o.Logger != nil {
		return o.Logger
	}
	return slog.Default()
}

func (o Options) maxRooms() int {
	if o.MaxRooms > 0 {
		return o.MaxRooms
	}
	return MaxRooms
}

type Room struct {
	name  string
	mgr   *Manager
	opts  Options
	inbox chan func()
	done  chan struct{}
	log   *slog.Logger

	// Everything below is touched only by the room goroutine.
	seats     [engine.NumSeats]seat
	host      engine.Seat
	dealer    engine.Seat
	game      *engine.Game
	rng       *mrand.Rand
	settings  Settings
	epoch     int                   // bumped whenever the game moves on; stale timers check it
	botQueued [engine.NumSeats]bool // a bot move is already scheduled for this epoch
	// Turn / claim clock (spec 4.6), armed once per epoch.
	timerEpoch int
	deadline   time.Time // zero when the current step has no time limit
	timeLimit  time.Duration
	connGen    int // bumped whenever a player connects or disconnects; stale idle timers check it
	closed     bool
}

func newRoom(name string, mgr *Manager, opts Options) *Room {
	seed := opts.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	r := &Room{
		name:     name,
		mgr:      mgr,
		opts:     opts,
		inbox:    make(chan func(), 64),
		done:     make(chan struct{}),
		log:      opts.logger().With("room", name),
		host:     -1,
		rng:      mrand.New(mrand.NewSource(seed)),
		settings: DefaultSettings(),
	}
	r.log.Info("room created")
	go r.loop()
	return r
}

func (r *Room) Name() string { return r.name }

func (r *Room) loop() {
	for {
		select {
		case f := <-r.inbox:
			f()
			if r.closed {
				return
			}
		case <-r.done:
			return
		}
	}
}

// post queues f to run on the room goroutine. It returns false if the room is closed.
func (r *Room) post(f func()) bool {
	select {
	case <-r.done:
		return false
	default:
	}
	select {
	case r.inbox <- f:
		return true
	case <-r.done:
		return false
	}
}

// join seats a human and returns the seat. It waits for the room goroutine.
func (r *Room) join(name string, c Client) (engine.Seat, error) {
	return r.call(func() (engine.Seat, error) { return r.doJoin(name, c) })
}

// rejoin gives a disconnected player their seat back. It waits for the room goroutine.
func (r *Room) rejoin(token string, c Client) (engine.Seat, error) {
	return r.call(func() (engine.Seat, error) { return r.doRejoin(token, c) })
}

// call runs f on the room goroutine and waits for its answer.
func (r *Room) call(f func() (engine.Seat, error)) (engine.Seat, error) {
	type reply struct {
		seat engine.Seat
		err  error
	}
	ch := make(chan reply, 1)
	ok := r.post(func() {
		s, err := f()
		ch <- reply{s, err}
	})
	if !ok {
		return -1, ErrRoomClosed
	}
	select {
	case x := <-ch:
		return x.seat, x.err
	case <-r.done:
		return -1, ErrRoomClosed
	}
}

func (r *Room) doJoin(name string, c Client) (engine.Seat, error) {
	if r.game != nil {
		return -1, ErrGameStarted
	}
	for s := range r.seats {
		if r.seats[s].kind == seatEmpty {
			r.seats[s] = seat{kind: seatHuman, name: name, token: newToken(), client: c}
			r.connGen++
			r.fixHost()
			r.log.Info("player joined", "seat", s, "name", name)
			r.broadcastRoom()
			return engine.Seat(s), nil
		}
	}
	return -1, ErrRoomFull
}

func (r *Room) doRejoin(token string, c Client) (engine.Seat, error) {
	s := engine.Seat(-1)
	for i := range r.seats {
		if token != "" && r.seats[i].kind == seatHuman && r.seats[i].token == token {
			s = engine.Seat(i)
		}
	}
	if s < 0 {
		r.log.Info("rejoin refused", "reason", "bad token")
		return -1, ErrBadToken
	}
	r.log.Info("player rejoined", "seat", s, "replaced_connection", r.seats[s].client != nil)
	if old := r.seats[s].client; old != nil {
		// The same player opened the game somewhere else; the newest connection wins.
		old.Send(Message{Type: MsgError, Data: ErrSessionMoved})
		old.Close()
	}
	wasFrozen := r.connectedHumans() == 0
	r.seats[s].client = c
	r.connGen++
	r.fixHost()
	if wasFrozen && r.game != nil && !r.game.Ended() {
		r.resume()
	}
	r.broadcastRoom()
	if r.game != nil {
		r.sendStateTo(s)
		r.sendClaimWindowTo(s)
	}
	r.scheduleBots()
	return s, nil
}

// AddBot fills an empty seat with a bot. Host only, before the game starts.
func (r *Room) AddBot(by, target engine.Seat) {
	r.post(func() {
		switch {
		case by != r.host:
			r.sendError(by, ErrNotHost)
		case r.game != nil:
			r.sendError(by, ErrGameStarted)
		case !target.Valid():
			r.sendError(by, ErrBadSeat)
		case r.seats[target].kind != seatEmpty:
			r.sendError(by, ErrSeatTaken)
		default:
			r.seats[target] = seat{kind: seatBot, name: botName(target), bot: bot.New(r.rng.Int63())}
			r.broadcastRoom()
		}
	})
}

// RemoveBot frees a seat that holds a bot. Host only, before the game starts.
func (r *Room) RemoveBot(by, target engine.Seat) {
	r.post(func() {
		switch {
		case by != r.host:
			r.sendError(by, ErrNotHost)
		case r.game != nil:
			r.sendError(by, ErrGameStarted)
		case !target.Valid():
			r.sendError(by, ErrBadSeat)
		case r.seats[target].kind != seatBot:
			r.sendError(by, ErrNotBot)
		default:
			r.seats[target] = seat{}
			r.broadcastRoom()
		}
	})
}

// UpdateSettings changes the room options. Host only, not while a round runs.
func (r *Room) UpdateSettings(by engine.Seat, st Settings) {
	r.post(func() {
		switch {
		case by != r.host:
			r.sendError(by, ErrNotHost)
		case r.game != nil && !r.game.Ended():
			r.sendError(by, ErrRoundRunning)
		case !st.Valid():
			r.sendError(by, ErrBadSettings)
		default:
			r.settings = st
			r.broadcastRoom()
		}
	})
}

// Start begins the first round. Host only; every seat must be filled.
func (r *Room) Start(by engine.Seat) {
	r.post(func() {
		switch {
		case by != r.host:
			r.sendError(by, ErrNotHost)
		case r.game != nil:
			r.sendError(by, ErrGameStarted)
		default:
			for _, s := range r.seats {
				if s.kind == seatEmpty {
					r.sendError(by, ErrRoomNotFull)
					return
				}
			}
			r.startRound(0)
		}
	})
}

// NextRound starts another round after the current one ended. Host only.
// After a win the server rolls the dice for the next dealer (spec 4.7);
// after a draw the dealer stays.
func (r *Room) NextRound(by engine.Seat) {
	r.post(func() {
		switch {
		case by != r.host:
			r.sendError(by, ErrNotHost)
		case r.game == nil:
			r.sendError(by, ErrNoGame)
		case !r.game.Ended():
			r.sendError(by, ErrRoundRunning)
		default:
			dealer := r.dealer
			if res, _ := r.game.Result(); !res.Draw {
				dice := engine.RollDice(r.rng)
				dealer = engine.NextDealer(res.Winner, dice)
				ev := EventData{Type: EventDiceRolled, Seat: res.Winner, Dice: &dice, NewDealer: &dealer}
				r.broadcast(Message{Type: MsgEvent, Data: ev})
			}
			r.startRound(dealer)
		}
	})
}

// Act applies a game action (discard, claim, pass, own kong, hu) for seat.
func (r *Room) Act(by engine.Seat, a engine.Action) {
	r.post(func() {
		if r.game == nil {
			r.sendError(by, ErrNoGame)
			return
		}
		events, err := r.game.Apply(by, a)
		if err != nil {
			r.sendError(by, engineError(err))
			return
		}
		r.afterApply(events)
	})
}

// Leave removes a human for good (they pressed "leave"). Before the game the
// seat is freed; during the game a bot takes the seat over. The room closes
// when no human is left.
func (r *Room) Leave(by engine.Seat) {
	r.post(func() {
		if !by.Valid() || r.seats[by].kind != seatHuman {
			return
		}
		r.log.Info("player left", "seat", by)
		if r.game == nil {
			r.seats[by] = seat{}
		} else {
			r.seats[by] = seat{kind: seatBot, name: r.seats[by].name + " (bot)", bot: bot.New(r.rng.Int63())}
		}
		r.connGen++
		r.afterSeatChange()
	})
}

// Disconnect is called when c's connection drops. The player keeps their
// seat and may rejoin with their token; meanwhile a bot plays for them (spec
// 4.6). In the waiting room the seat is kept for ReconnectGrace only.
func (r *Room) Disconnect(by engine.Seat, c Client) {
	r.post(func() {
		if !by.Valid() || r.seats[by].kind != seatHuman || r.seats[by].client != c {
			return // already left, or replaced by a newer connection
		}
		r.log.Info("player disconnected", "seat", by)
		st := &r.seats[by]
		st.client = nil
		if st.bot == nil {
			st.bot = bot.New(r.rng.Int63())
		}
		r.connGen++
		if r.game == nil {
			token := st.token
			r.after(ReconnectGrace, func() { r.expireAway(by, token) })
		}
		r.afterSeatChange()
	})
}

// expireAway frees a waiting-room seat whose player did not come back in time.
func (r *Room) expireAway(s engine.Seat, token string) {
	st := &r.seats[s]
	if r.game != nil || st.kind != seatHuman || st.client != nil || st.token != token {
		return
	}
	r.log.Info("seat freed: player did not come back", "seat", s)
	r.seats[s] = seat{}
	r.afterSeatChange()
}

// afterSeatChange fixes the host, closes or freezes the room as needed, and
// tells everyone.
func (r *Room) afterSeatChange() {
	r.fixHost()
	if r.host < 0 {
		r.closeRoom("no players left")
		return
	}
	if r.connectedHumans() == 0 {
		r.freeze()
		gen := r.connGen
		r.after(IdleRoomTimeout, func() {
			if gen == r.connGen && r.connectedHumans() == 0 {
				r.closeRoom("idle")
			}
		})
		return
	}
	r.broadcastRoom()
	r.scheduleBots()
}

// fixHost keeps the host on a connected human when there is one. A host who
// is only away keeps the role while nobody else is connected.
func (r *Room) fixHost() {
	if r.host >= 0 && r.seats[r.host].connected() {
		return
	}
	for s := range r.seats {
		if r.seats[s].connected() {
			r.host = engine.Seat(s)
			return
		}
	}
	if r.host >= 0 && r.seats[r.host].kind == seatHuman {
		return
	}
	r.host = -1
	for s := range r.seats {
		if r.seats[s].kind == seatHuman {
			r.host = engine.Seat(s)
			return
		}
	}
}

func (r *Room) connectedHumans() int {
	n := 0
	for s := range r.seats {
		if r.seats[s].connected() {
			n++
		}
	}
	return n
}

// freeze stops the game clock and bot moves while nobody is connected, so the
// round does not play itself out while everyone is reconnecting.
func (r *Room) freeze() {
	if r.game != nil && !r.game.Ended() {
		r.log.Info("round paused: nobody connected")
	}
	r.epoch++
	r.botQueued = [engine.NumSeats]bool{}
	r.timerEpoch = r.epoch
	r.deadline, r.timeLimit = time.Time{}, 0
}

// resume restarts the clock (a fresh full turn) and the bots after a freeze.
func (r *Room) resume() {
	r.epoch++
	r.botQueued = [engine.NumSeats]bool{}
	r.armTimer()
}

func (r *Room) closeRoom(reason string) {
	r.log.Info("room closed", "reason", reason)
	r.closed = true
	close(r.done)
	r.mgr.remove(r)
}

// after runs f on the room goroutine once d (in room time, see Options.TimeUnit) has passed.
func (r *Room) after(d time.Duration, f func()) {
	time.AfterFunc(r.scaled(d), func() { r.post(f) })
}

// scaled converts a duration in real seconds into room time.
func (r *Room) scaled(d time.Duration) time.Duration {
	if r.opts.TimeUnit == 0 {
		return d
	}
	return d / time.Second * r.opts.TimeUnit
}

func (r *Room) startRound(dealer engine.Seat) {
	r.dealer = dealer
	r.game = engine.NewGame(r.rng.Int63(), dealer)
	r.log.Info("round started", "dealer", dealer, "humans", r.connectedHumans())
	r.epoch++
	r.botQueued = [engine.NumSeats]bool{}
	r.armTimer()
	r.broadcastRoom()
	r.sendStates()
	r.scheduleBots()
}

// afterApply tells every human what happened and lets bots respond.
func (r *Room) afterApply(events []engine.Event) {
	if len(events) > 0 {
		// The game moved on (a claim window that is still collecting answers
		// produces no events); any bot move scheduled before is now stale.
		r.epoch++
		r.botQueued = [engine.NumSeats]bool{}
	}
	r.armTimer()
	for s := range r.seats {
		c := r.seats[s].client
		if c == nil {
			continue
		}
		for _, e := range events {
			c.Send(Message{Type: MsgEvent, Data: r.eventFor(e, engine.Seat(s))})
		}
	}
	claimOpened := false
	for _, e := range events {
		claimOpened = claimOpened || e.Type == engine.EventClaimWindow
	}
	if r.game.Ended() {
		if res, ok := r.game.Result(); ok {
			if res.Draw {
				r.log.Info("round ended", "draw", true)
			} else {
				r.log.Info("round ended", "winner", res.Winner, "self_draw", res.SelfDraw)
			}
		}
		r.broadcastRoom()
	}
	r.sendStates()
	if claimOpened {
		r.sendClaimWindows()
	}
	r.scheduleBots()
}

func (r *Room) eventFor(e engine.Event, viewer engine.Seat) EventData {
	e = e.ForSeat(viewer)
	d := EventData{Type: string(e.Type), Seat: e.Seat, Tiles: e.Tiles, Meld: e.Meld}
	if e.Type == engine.EventGameEnd {
		if res, ok := r.game.Result(); ok {
			d.Result = &res
		}
	}
	return d
}

func (r *Room) sendStates() {
	for s := range r.seats {
		r.sendStateTo(engine.Seat(s))
	}
}

func (r *Room) sendStateTo(s engine.Seat) {
	if c := r.seats[s].client; c != nil {
		c.Send(Message{Type: MsgGameState, Data: GameState{
			PlayerView:  r.game.ViewFor(s),
			TimeLeftMs:  r.timeLeftMs(),
			TimeLimitMs: int(r.timeLimit / time.Millisecond),
		}})
	}
}

func (r *Room) sendClaimWindows() {
	for s := range r.seats {
		r.sendClaimWindowTo(engine.Seat(s))
	}
}

func (r *Room) sendClaimWindowTo(s engine.Seat) {
	c := r.seats[s].client
	if c == nil || r.game.Phase() != engine.PhaseClaim {
		return
	}
	v := r.game.ViewFor(s)
	if len(v.LegalActions) == 0 || v.LastDiscard == nil {
		return
	}
	c.Send(Message{Type: MsgClaimWindow, Data: ClaimWindowData{
		Tile: *v.LastDiscard, From: v.LastDiscarder, Actions: v.LegalActions, TimeLeftMs: r.timeLeftMs(),
	}})
}

// scheduleBots queues a delayed move for every bot the game is waiting on.
func (r *Room) scheduleBots() {
	if r.game == nil || r.game.Ended() || r.connectedHumans() == 0 {
		return
	}
	for _, s := range r.game.Pending() {
		if !r.seats[s].botPlays() || r.botQueued[s] {
			continue
		}
		r.botQueued[s] = true
		s, epoch := s, r.epoch
		time.AfterFunc(r.botDelay(), func() {
			r.post(func() { r.botMove(s, epoch) })
		})
	}
}

func (r *Room) botMove(s engine.Seat, epoch int) {
	if epoch != r.epoch || r.game == nil || r.game.Ended() || !r.seats[s].botPlays() {
		return
	}
	r.botQueued[s] = false
	v := r.game.ViewFor(s)
	if len(v.LegalActions) == 0 {
		return
	}
	a := r.seats[s].bot.Choose(v)
	events, err := r.game.Apply(s, a)
	if err != nil {
		// Should never happen: the bot only picks legal actions.
		r.log.Error("bot chose an illegal action", "seat", s, "action", a.Type, "err", err)
		if events, err = r.game.Apply(s, v.LegalActions[len(v.LegalActions)-1]); err != nil {
			return
		}
	}
	r.afterApply(events)
}

func (r *Room) botDelay() time.Duration {
	if r.opts.BotDelay != nil {
		return r.opts.BotDelay(r.rng)
	}
	lo, hi := r.settings.BotDelayMinMs, r.settings.BotDelayMaxMs
	return time.Duration(lo+r.rng.Intn(hi-lo+1)) * time.Millisecond
}

// armTimer starts the clock for the current step (a turn or a claim window)
// unless it already runs for this epoch.
func (r *Room) armTimer() {
	if r.timerEpoch == r.epoch {
		return
	}
	r.timerEpoch = r.epoch
	r.deadline, r.timeLimit = time.Time{}, 0
	if r.game == nil || r.game.Ended() || r.connectedHumans() == 0 {
		return
	}
	limit := r.scaled(time.Duration(r.settings.TurnSeconds) * time.Second)
	if r.game.Phase() == engine.PhaseClaim {
		limit = r.scaled(time.Duration(r.settings.ClaimSeconds) * time.Second)
	}
	if limit <= 0 {
		return
	}
	r.deadline, r.timeLimit = time.Now().Add(limit), limit
	epoch := r.epoch
	time.AfterFunc(limit, func() {
		r.post(func() { r.onTimeout(epoch) })
	})
}

func (r *Room) timeLeftMs() int {
	if r.deadline.IsZero() {
		return 0
	}
	return max(int(time.Until(r.deadline)/time.Millisecond), 1)
}

// onTimeout plays for whoever ran out of time: unanswered claims count as a
// pass, and a player who did not discard in time throws the tile they just
// drew (or their last tile after a chi/pung).
func (r *Room) onTimeout(epoch int) {
	if epoch != r.epoch || r.game == nil || r.game.Ended() {
		return
	}
	var events []engine.Event
	var err error
	if r.game.Phase() == engine.PhaseClaim {
		events, err = r.game.ExpireClaims()
	} else {
		s := r.game.Turn()
		events, err = r.game.Apply(s, autoDiscard(r.game.ViewFor(s)))
	}
	if err != nil {
		r.log.Error("timeout move failed", "err", err)
		return
	}
	r.afterApply(events)
}

func autoDiscard(v engine.PlayerView) engine.Action {
	var last engine.Action
	for _, a := range v.LegalActions {
		if a.Type != engine.ActionDiscard {
			continue
		}
		if v.Drawn != nil && a.TileID == v.Drawn.ID {
			return a
		}
		last = a
	}
	return last
}

func (r *Room) status() string {
	switch {
	case r.game == nil:
		return "waiting"
	case r.game.Ended():
		return "ended"
	}
	return "playing"
}

func (r *Room) broadcastRoom() {
	st := RoomState{Room: r.name, Status: r.status(), Host: r.host, Dealer: r.dealer, Settings: r.settings}
	for s := range r.seats {
		x := &r.seats[s]
		st.Seats[s] = SeatInfo{Seat: engine.Seat(s), Kind: seatKindNames[x.kind], Name: x.name, Connected: x.connected()}
	}
	for s := range r.seats {
		if c := r.seats[s].client; c != nil {
			st.YourSeat = engine.Seat(s)
			st.Token = r.seats[s].token
			c.Send(Message{Type: MsgRoomState, Data: st})
		}
	}
}

func (r *Room) broadcast(m Message) {
	for s := range r.seats {
		if c := r.seats[s].client; c != nil {
			c.Send(m)
		}
	}
}

func (r *Room) sendError(s engine.Seat, e *Error) {
	if s.Valid() && r.seats[s].client != nil {
		r.seats[s].client.Send(Message{Type: MsgError, Data: e})
	}
}

func newToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand does not fail on supported platforms
	}
	return hex.EncodeToString(b)
}

func botName(s engine.Seat) string {
	return fmt.Sprintf("Bot %d", int(s)+1)
}
