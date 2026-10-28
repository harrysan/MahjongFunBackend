package engine

import (
	"errors"
	"math/rand"
	"sort"
)

var (
	ErrGameEnded     = errors.New("game already ended")
	ErrNotYourTurn   = errors.New("not your turn")
	ErrTileNotInHand = errors.New("tile not in hand")
	ErrIllegalAction = errors.New("illegal action")
)

// Phase is the stage of a round.
type Phase uint8

const (
	PhaseDiscarding Phase = iota // current player holds 3n+2 tiles and must act (discard, kong, or hu)
	PhaseClaim                   // a tile was just discarded; other players may claim it
	PhaseEnded
)

type ActionType uint8

const (
	ActionDiscard ActionType = iota + 1 // TileID = tile to discard
	ActionHu                            // win (self-draw, or on the discarded tile during a claim)
	ActionChi                           // claim: TileID and TileID2 = the two hand tiles to use
	ActionPung                          // claim the discarded tile
	ActionKong                          // claim the discard, or (own turn) TileID = a tile of the kong
	ActionPass                          // decline to claim
)

type Action struct {
	Type    ActionType `json:"type"`
	TileID  uint8      `json:"tile_id"`
	TileID2 uint8      `json:"tile_id2"`
}

type EventType string

const (
	EventDrew        EventType = "drew"         // Tiles = drawn tile (hidden from other seats, see ForSeat)
	EventBonus       EventType = "bonus"        // Tiles = bonus tile set aside
	EventDiscarded   EventType = "discarded"    // Tiles = discarded tile
	EventClaimWindow EventType = "claim_window" // Seat = discarder, Tiles = the discarded tile
	EventClaimed     EventType = "claimed"      // Seat = claimer, Meld = type, Tiles = meld tiles
	EventTurn        EventType = "turn"         // Seat = whose turn it is now
	EventGameEnd     EventType = "game_end"     // see Game.Result
)

type Event struct {
	Type  EventType `json:"type"`
	Seat  Seat      `json:"seat"`
	Tiles []Tile    `json:"tiles,omitempty"`
	Meld  MeldType  `json:"meld,omitempty"`
}

// ForSeat returns the event as `viewer` may see it: other players' draws and
// concealed kongs are hidden.
func (e Event) ForSeat(viewer Seat) Event {
	if e.Seat != viewer {
		if e.Type == EventDrew || (e.Type == EventClaimed && e.Meld == MeldConcealedKong) {
			e.Tiles = nil
		}
	}
	return e
}

// Result describes how a round ended.
type Result struct {
	Draw     bool `json:"draw"` // wall exhausted, nobody won
	Winner   Seat `json:"winner"`
	From     Seat `json:"from"` // who discarded the winning tile (when SelfDraw is false)
	SelfDraw bool `json:"self_draw"`
	WinTile  Tile `json:"win_tile"`
}

type MeldType uint8

const (
	MeldChi MeldType = iota + 1
	MeldPung
	MeldKong
	MeldConcealedKong
)

type Meld struct {
	Type  MeldType `json:"type"`
	Tiles []Tile   `json:"tiles"`
	From  Seat     `json:"from"`
}

type player struct {
	hand     []Tile
	melds    []Meld
	bonus    []Tile
	discards []Tile
}

// Game is the pure rules state machine for one round. It does no I/O and
// starts no goroutines; the room layer owns timers and concurrency.
type Game struct {
	wall      *Wall
	players   [NumSeats]player
	dealer    Seat
	turn      Seat // whose turn it is; during a claim window this stays the discarder
	phase     Phase
	drawn     Tile // last tile the current player drew
	canSelfHu bool // false right after a chi/pung claim (no draw happened)
	claim     *claimWindow
	result    *Result
}

// NewGame shuffles with seed, deals 13 tiles to each seat, replaces bonus
// tiles, and gives the dealer a 14th tile. The dealer plays first.
func NewGame(seed int64, dealer Seat) *Game {
	rng := rand.New(rand.NewSource(seed))
	g := &Game{wall: NewWall(rng), dealer: dealer, turn: dealer, phase: PhaseDiscarding}

	for i := 0; i < NumSeats; i++ {
		seat := Seat((int(dealer) + i) % NumSeats)
		for n := 0; n < 13; n++ {
			t, _ := g.wall.DrawFront()
			g.players[seat].hand = append(g.players[seat].hand, t)
		}
	}
	var scratch []Event
	for i := 0; i < NumSeats; i++ {
		seat := Seat((int(dealer) + i) % NumSeats)
		g.settleBonus(seat, &scratch)
	}
	g.draw(dealer, false, &scratch)
	return g
}

func (g *Game) Turn() Seat    { return g.turn }
func (g *Game) Dealer() Seat  { return g.dealer }
func (g *Game) Phase() Phase  { return g.phase }
func (g *Game) Ended() bool   { return g.phase == PhaseEnded }
func (g *Game) WallLeft() int { return g.wall.Remaining() }

// Result returns how the round ended. ok is false while the round is running.
func (g *Game) Result() (r Result, ok bool) {
	if g.result == nil {
		return Result{}, false
	}
	return *g.result, true
}

// Apply performs an action for seat. On error the state is unchanged.
// In PhaseDiscarding only the current player may act; in PhaseClaim every
// seat listed by Pending may answer (claim or pass) in any order.
func (g *Game) Apply(seat Seat, a Action) ([]Event, error) {
	if g.phase == PhaseEnded {
		return nil, ErrGameEnded
	}
	if !seat.Valid() {
		return nil, ErrNotYourTurn
	}
	if g.phase == PhaseClaim {
		return g.applyClaimResponse(seat, a)
	}
	if seat != g.turn {
		return nil, ErrNotYourTurn
	}
	switch a.Type {
	case ActionDiscard:
		return g.applyDiscard(seat, a.TileID)
	case ActionHu:
		return g.applyHuSelf(seat)
	case ActionKong:
		return g.applyKongSelf(seat, a.TileID)
	}
	return nil, ErrIllegalAction
}

// LegalActions lists what seat may do right now.
// Discarding phase: hu (if complete and allowed), kongs, then one discard per tile.
// Claim phase: hu / pung / kong / chi options for that seat, then pass.
func (g *Game) LegalActions(seat Seat) []Action {
	if !seat.Valid() {
		return nil
	}
	switch g.phase {
	case PhaseDiscarding:
		if seat != g.turn {
			return nil
		}
		p := &g.players[seat]
		acts := make([]Action, 0, len(p.hand)+2)
		if g.canSelfHu && IsWinning(CountsOf(p.hand), len(p.melds)) {
			acts = append(acts, Action{Type: ActionHu})
		}
		for _, id := range g.selfKongOptions(seat) {
			acts = append(acts, Action{Type: ActionKong, TileID: id})
		}
		for _, t := range p.hand {
			acts = append(acts, Action{Type: ActionDiscard, TileID: t.ID})
		}
		return acts
	case PhaseClaim:
		w := g.claim
		if w == nil || seat == w.discarder || w.responded[seat] || !w.opts[seat].has() {
			return nil
		}
		o := w.opts[seat]
		var acts []Action
		if o.hu {
			acts = append(acts, Action{Type: ActionHu})
		}
		if o.pung {
			acts = append(acts, Action{Type: ActionPung})
		}
		if o.kong {
			acts = append(acts, Action{Type: ActionKong})
		}
		if o.chi {
			for _, pr := range g.chiOptions(seat, w.tile) {
				acts = append(acts, Action{Type: ActionChi, TileID: pr[0].ID, TileID2: pr[1].ID})
			}
		}
		return append(acts, Action{Type: ActionPass})
	}
	return nil
}

// Pending lists the seats that must act now: the current player while
// discarding, or every seat that can still answer during a claim window.
func (g *Game) Pending() []Seat {
	switch g.phase {
	case PhaseDiscarding:
		return []Seat{g.turn}
	case PhaseClaim:
		w := g.claim
		var out []Seat
		for s := Seat(0); s < NumSeats; s++ {
			if s != w.discarder && w.opts[s].has() && !w.responded[s] {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func (g *Game) applyDiscard(seat Seat, id uint8) ([]Event, error) {
	p := &g.players[seat]
	t, ok := removeByID(p, id)
	if !ok {
		return nil, ErrTileNotInHand
	}
	p.discards = append(p.discards, t)

	events := []Event{{Type: EventDiscarded, Seat: seat, Tiles: []Tile{t}}}

	g.openClaimWindow(seat, t)
	if g.claim == nil {
		g.nextTurn(&events)
		return events, nil
	}
	g.phase = PhaseClaim
	events = append(events, Event{Type: EventClaimWindow, Seat: seat, Tiles: []Tile{t}})
	return events, nil
}

func (g *Game) applyHuSelf(seat Seat) ([]Event, error) {
	p := &g.players[seat]
	if !g.canSelfHu || !IsWinning(CountsOf(p.hand), len(p.melds)) {
		return nil, ErrIllegalAction
	}
	g.phase = PhaseEnded
	g.result = &Result{Winner: seat, From: seat, SelfDraw: true, WinTile: g.drawn}
	return []Event{{Type: EventGameEnd, Seat: seat}}, nil
}

// nextTurn passes play to the seat after g.turn, who draws a tile.
func (g *Game) nextTurn(events *[]Event) {
	next := g.turn.Next()
	g.turn = next
	g.phase = PhaseDiscarding
	if !g.draw(next, false, events) {
		g.endDraw(events)
		return
	}
	*events = append(*events, Event{Type: EventTurn, Seat: next})
}

// draw gives seat one tile and settles any bonus tiles. It returns false if the wall ran out.
func (g *Game) draw(seat Seat, fromBack bool, events *[]Event) bool {
	var t Tile
	var ok bool
	if fromBack {
		t, ok = g.wall.DrawBack()
	} else {
		t, ok = g.wall.DrawFront()
	}
	if !ok {
		return false
	}
	g.players[seat].hand = append(g.players[seat].hand, t)
	g.drawn = t
	g.canSelfHu = true
	*events = append(*events, Event{Type: EventDrew, Seat: seat, Tiles: []Tile{t}})
	return g.settleBonus(seat, events)
}

// settleBonus moves every bonus tile in seat's hand to its bonus area and
// draws a replacement from the back of the wall for each. Returns false if the wall ran out.
func (g *Game) settleBonus(seat Seat, events *[]Event) bool {
	p := &g.players[seat]
	for {
		idx := -1
		for i, t := range p.hand {
			if t.Kind.IsBonus() {
				idx = i
				break
			}
		}
		if idx < 0 {
			return true
		}
		b := p.hand[idx]
		p.hand = append(p.hand[:idx], p.hand[idx+1:]...)
		p.bonus = append(p.bonus, b)
		*events = append(*events, Event{Type: EventBonus, Seat: seat, Tiles: []Tile{b}})

		r, ok := g.wall.DrawBack()
		if !ok {
			return false
		}
		p.hand = append(p.hand, r)
		g.drawn = r
		*events = append(*events, Event{Type: EventDrew, Seat: seat, Tiles: []Tile{r}})
	}
}

func (g *Game) endDraw(events *[]Event) {
	g.phase = PhaseEnded
	g.claim = nil
	g.result = &Result{Draw: true}
	*events = append(*events, Event{Type: EventGameEnd, Seat: g.turn})
}

// PlayerView is everything one player is allowed to know.
type PlayerView struct {
	Seat          Seat             `json:"seat"`
	Phase         Phase            `json:"phase"`
	Turn          Seat             `json:"turn"`
	Dealer        Seat             `json:"dealer"`
	Hand          []Tile           `json:"hand"`            // sorted; includes the just-drawn tile
	Drawn         *Tile            `json:"drawn,omitempty"` // set only when it is your turn and you just drew
	Melds         [NumSeats][]Meld `json:"melds"`
	Bonus         [NumSeats][]Tile `json:"bonus"`
	Discards      [NumSeats][]Tile `json:"discards"`
	HandCounts    [NumSeats]int    `json:"hand_counts"`
	WallRemaining int              `json:"wall_remaining"`
	LegalActions  []Action         `json:"legal_actions"`
	Pending       []Seat           `json:"pending"`                 // seats that must act now
	LastDiscard   *Tile            `json:"last_discard,omitempty"`  // set during a claim window
	LastDiscarder Seat             `json:"last_discarder"`          // meaningful during a claim window
	Result        *Result          `json:"result,omitempty"`
}

// ViewFor builds the snapshot for seat. Other players' hands are never included.
func (g *Game) ViewFor(seat Seat) PlayerView {
	v := PlayerView{
		Phase:         g.phase,
		Turn:          g.turn,
		Dealer:        g.dealer,
		WallRemaining: g.wall.Remaining(),
		Pending:       g.Pending(),
	}
	if !seat.Valid() {
		return v
	}
	v.Seat = seat
	v.LegalActions = g.LegalActions(seat)
	for s := 0; s < NumSeats; s++ {
		p := &g.players[s]
		v.HandCounts[s] = len(p.hand)
		v.Melds[s] = cloneMelds(p.melds)
		v.Bonus[s] = cloneTiles(p.bonus)
		v.Discards[s] = cloneTiles(p.discards)
	}
	hand := cloneTiles(g.players[seat].hand)
	sort.Slice(hand, func(i, j int) bool {
		if hand[i].Kind != hand[j].Kind {
			return hand[i].Kind < hand[j].Kind
		}
		return hand[i].ID < hand[j].ID
	})
	v.Hand = hand
	if g.phase == PhaseDiscarding && seat == g.turn && g.canSelfHu {
		d := g.drawn
		v.Drawn = &d
	}
	if g.phase == PhaseClaim && g.claim != nil {
		t := g.claim.tile
		v.LastDiscard = &t
		v.LastDiscarder = g.claim.discarder
	}
	if g.result != nil {
		r := *g.result
		v.Result = &r
	}
	return v
}

func cloneTiles(ts []Tile) []Tile {
	out := make([]Tile, len(ts))
	copy(out, ts)
	return out
}

func cloneMelds(ms []Meld) []Meld {
	out := make([]Meld, len(ms))
	for i, m := range ms {
		out[i] = Meld{Type: m.Type, Tiles: cloneTiles(m.Tiles), From: m.From}
	}
	return out
}
