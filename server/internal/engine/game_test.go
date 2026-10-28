package engine

import (
	"errors"
	"math/rand"
	"testing"
)

func TestNewGameDeal(t *testing.T) {
	for seed := int64(1); seed <= 50; seed++ {
		for dealer := Seat(0); dealer < NumSeats; dealer++ {
			g := NewGame(seed, dealer)
			for s := Seat(0); s < NumSeats; s++ {
				p := &g.players[s]
				want := 13
				if s == dealer {
					want = 14
				}
				if len(p.hand) != want {
					t.Fatalf("seed %d dealer %d: seat %d has %d tiles, want %d", seed, dealer, s, len(p.hand), want)
				}
				for _, tile := range p.hand {
					if tile.Kind.IsBonus() {
						t.Fatalf("seed %d: seat %d still holds bonus tile %v", seed, s, tile)
					}
				}
				for _, tile := range p.bonus {
					if !tile.Kind.IsBonus() {
						t.Fatalf("seed %d: non-bonus tile %v in bonus area", seed, tile)
					}
				}
			}
			if g.Turn() != dealer || g.Phase() != PhaseDiscarding || g.Ended() {
				t.Fatalf("bad initial state: turn=%d phase=%d", g.Turn(), g.Phase())
			}
			checkConservation(t, g)
		}
	}
}

func TestNewGameDeterministic(t *testing.T) {
	a, b := NewGame(99, 0), NewGame(99, 0)
	for s := 0; s < NumSeats; s++ {
		if len(a.players[s].hand) != len(b.players[s].hand) {
			t.Fatalf("seat %d hand sizes differ", s)
		}
		for i := range a.players[s].hand {
			if a.players[s].hand[i] != b.players[s].hand[i] {
				t.Fatalf("seat %d tile %d differs", s, i)
			}
		}
	}
}

func TestDiscardAdvancesTurn(t *testing.T) {
	g := NewGame(7, 0)
	tile := g.players[0].hand[0]
	events := discardAndSettle(t, g, 0, tile.ID)
	if events[0].Type != EventDiscarded || events[0].Tiles[0] != tile {
		t.Errorf("first event = %+v, want discard of %v", events[0], tile)
	}
	if g.Turn() != 1 || g.Phase() != PhaseDiscarding {
		t.Errorf("turn = %d phase = %d, want turn 1 discarding", g.Turn(), g.Phase())
	}
	if len(g.players[0].hand) != 13 || len(g.players[1].hand) != 14 {
		t.Errorf("hand sizes after discard: %d / %d", len(g.players[0].hand), len(g.players[1].hand))
	}
	if len(g.players[0].discards) != 1 || g.players[0].discards[0] != tile {
		t.Errorf("discard pile wrong: %v", g.players[0].discards)
	}
	checkConservation(t, g)
}

func TestApplyRejectsBadActions(t *testing.T) {
	g := NewGame(7, 0)
	other := g.players[1].hand[0]

	if _, err := g.Apply(1, Action{Type: ActionDiscard, TileID: other.ID}); !errors.Is(err, ErrNotYourTurn) {
		t.Errorf("wrong seat: err = %v, want ErrNotYourTurn", err)
	}
	if _, err := g.Apply(0, Action{Type: ActionDiscard, TileID: other.ID}); !errors.Is(err, ErrTileNotInHand) {
		t.Errorf("tile of another player: err = %v, want ErrTileNotInHand", err)
	}
	if _, err := g.Apply(0, Action{Type: 99}); !errors.Is(err, ErrIllegalAction) {
		t.Errorf("unknown action: err = %v, want ErrIllegalAction", err)
	}
	if g.Turn() != 0 || len(g.players[0].hand) != 14 {
		t.Error("rejected actions changed the state")
	}
}

func TestHuSelfDraw(t *testing.T) {
	g := NewGame(1, 0)
	g.players[0].hand = tilesFor(t, "123d 456d 789d 123b 55c")
	g.drawn = g.players[0].hand[13]

	acts := g.LegalActions(0)
	if len(acts) == 0 || acts[0].Type != ActionHu {
		t.Fatalf("hu should be the first legal action, got %v", acts)
	}
	if _, err := g.Apply(0, Action{Type: ActionHu}); err != nil {
		t.Fatal(err)
	}
	res, ok := g.Result()
	if !ok || res.Draw || res.Winner != 0 || !res.SelfDraw || res.WinTile != g.drawn {
		t.Errorf("result = %+v ok=%v", res, ok)
	}
	if !g.Ended() {
		t.Error("game should be over")
	}
	if _, err := g.Apply(0, Action{Type: ActionDiscard, TileID: 0}); !errors.Is(err, ErrGameEnded) {
		t.Errorf("after the end: err = %v, want ErrGameEnded", err)
	}
	if g.LegalActions(0) != nil {
		t.Error("no legal actions after the game ended")
	}
}

func TestHuRejectedWhenNotComplete(t *testing.T) {
	g := NewGame(1, 0)
	g.players[0].hand = tilesFor(t, "123d 456d 789d 123b 56c")
	if _, err := g.Apply(0, Action{Type: ActionHu}); !errors.Is(err, ErrIllegalAction) {
		t.Errorf("err = %v, want ErrIllegalAction", err)
	}
	if g.Ended() {
		t.Error("game must not end on a false hu")
	}
	for _, a := range g.LegalActions(0) {
		if a.Type == ActionHu {
			t.Error("hu offered for an incomplete hand")
		}
	}
}

func TestBonusTileIsReplacedFromBack(t *testing.T) {
	// Find a seed where flower 1 is still on the wall (not in anyone's opening
	// hand), move it to the front, and make sure the tile at the back is a normal tile.
	flower := NewTile(136)
	var g *Game
	var replacement Tile
	for seed := int64(1); seed <= 500 && g == nil; seed++ {
		cand := NewGame(seed, 0)
		pos := -1
		for i := cand.wall.front; i <= cand.wall.back; i++ {
			if cand.wall.tiles[i].ID == flower.ID {
				pos = i
				break
			}
		}
		if pos < 0 {
			continue
		}
		front := cand.wall.front
		cand.wall.tiles[pos], cand.wall.tiles[front] = cand.wall.tiles[front], cand.wall.tiles[pos]
		if cand.wall.tiles[cand.wall.back].Kind.IsBonus() {
			continue
		}
		g = cand
		replacement = cand.wall.tiles[cand.wall.back]
	}
	if g == nil {
		t.Fatal("no seed found with a flower on the wall")
	}
	bonusBefore := len(g.players[1].bonus)

	// Seat 0 discards; seat 1 then draws the flower from the front.
	events := discardAndSettle(t, g, 0, g.players[0].hand[0].ID)
	if len(g.players[1].bonus) != bonusBefore+1 {
		t.Errorf("flower was not set aside")
	}
	if len(g.players[1].hand) != 14 {
		t.Errorf("hand has %d tiles, want 14", len(g.players[1].hand))
	}
	found := false
	for _, tile := range g.players[1].hand {
		if tile.ID == replacement.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("replacement tile %v from the back of the wall is not in hand", replacement)
	}
	hasBonusEvent := false
	for _, e := range events {
		if e.Type == EventBonus && e.Seat == 1 {
			hasBonusEvent = true
		}
	}
	if !hasBonusEvent {
		t.Error("no bonus event emitted")
	}
	checkConservation(t, g)
}

func TestWallExhaustedIsDraw(t *testing.T) {
	g := NewGame(3, 0)
	g.wall.front = g.wall.back + 1 // nothing left to draw
	// Tiles drawn so far stay where they are, so skip the conservation check here.
	events := discardAndSettle(t, g, 0, g.players[0].hand[0].ID)
	res, ok := g.Result()
	if !ok || !res.Draw || !g.Ended() {
		t.Fatalf("expected a drawn round, got %+v ok=%v", res, ok)
	}
	if events[len(events)-1].Type != EventGameEnd {
		t.Errorf("last event = %v, want game_end", events[len(events)-1].Type)
	}
}

func TestViewHidesOtherHands(t *testing.T) {
	g := NewGame(5, 2)
	v := g.ViewFor(0)
	if len(v.Hand) != 13 {
		t.Errorf("seat 0 hand = %d tiles, want 13", len(v.Hand))
	}
	if v.Drawn != nil {
		t.Error("Drawn should only be set for the seat whose turn it is")
	}
	if v.HandCounts[2] != 14 || v.HandCounts[1] != 13 {
		t.Errorf("hand counts = %v", v.HandCounts)
	}
	if v.LegalActions != nil {
		t.Error("seat 0 has no legal actions on seat 2's turn")
	}
	dv := g.ViewFor(2)
	if dv.Drawn == nil || len(dv.Hand) != 14 {
		t.Errorf("dealer view wrong: drawn=%v hand=%d", dv.Drawn, len(dv.Hand))
	}
	for i := 1; i < len(dv.Hand); i++ {
		if dv.Hand[i-1].Kind > dv.Hand[i].Kind {
			t.Fatal("hand is not sorted")
		}
	}
	if bad := g.ViewFor(7); bad.Hand != nil {
		t.Error("invalid seat should get an empty view")
	}
}

func TestEventRedaction(t *testing.T) {
	e := Event{Type: EventDrew, Seat: 1, Tiles: []Tile{NewTile(5)}}
	if got := e.ForSeat(1); len(got.Tiles) != 1 {
		t.Error("the drawing player must see the tile")
	}
	if got := e.ForSeat(0); got.Tiles != nil {
		t.Error("other players must not see the drawn tile")
	}
	d := Event{Type: EventDiscarded, Seat: 1, Tiles: []Tile{NewTile(5)}}
	if got := d.ForSeat(0); len(got.Tiles) != 1 {
		t.Error("discards are public")
	}
}

// Plays many complete random games: random legal actions for random pending
// seats, hu whenever it is offered. Exercises claims, kongs and bonus tiles together.
func TestSimulation(t *testing.T) {
	draws, selfWins, discardWins := 0, 0, 0
	melds := map[MeldType]int{}
	for seed := int64(1); seed <= 1000; seed++ {
		rng := rand.New(rand.NewSource(seed))
		g := NewGame(seed, Seat(seed%NumSeats))
		for steps := 0; !g.Ended(); steps++ {
			if steps > 3000 {
				t.Fatalf("seed %d: game did not finish", seed)
			}
			pending := g.Pending()
			if len(pending) == 0 {
				t.Fatalf("seed %d: nobody is pending but the game is not over", seed)
			}
			seat := pending[rng.Intn(len(pending))]
			acts := g.LegalActions(seat)
			if len(acts) == 0 {
				t.Fatalf("seed %d: no legal actions for pending seat %d", seed, seat)
			}
			act := acts[rng.Intn(len(acts))]
			if acts[0].Type == ActionHu {
				act = acts[0]
			}
			events, err := g.Apply(seat, act)
			if err != nil {
				t.Fatalf("seed %d: legal action %+v rejected: %v", seed, act, err)
			}
			for _, e := range events {
				if e.Type == EventClaimed {
					melds[e.Meld]++
				}
			}
			checkHandSizes(t, g)
		}
		checkConservation(t, g)
		res, ok := g.Result()
		if !ok {
			t.Fatalf("seed %d: ended without a result", seed)
		}
		if res.Draw {
			draws++
			continue
		}
		if res.SelfDraw {
			selfWins++
		} else {
			discardWins++
		}
		p := &g.players[res.Winner]
		if !IsWinning(CountsOf(p.hand), len(p.melds)) {
			t.Fatalf("seed %d: winner %d does not hold a complete hand", seed, res.Winner)
		}
	}
	t.Logf("1000 games: %d draws, %d self-draw wins, %d wins on a discard; melds chi=%d pung=%d kong=%d concealed=%d",
		draws, selfWins, discardWins, melds[MeldChi], melds[MeldPung], melds[MeldKong], melds[MeldConcealedKong])
}
