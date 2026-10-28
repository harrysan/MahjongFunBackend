package engine

import (
	"errors"
	"slices"
	"testing"
)

// claimTable builds a table where seat 0 (dealer) discards a 5 of dots and the
// other seats hold the given hands (13 tiles each). It returns the game in
// PhaseClaim; the test fails if nobody can claim.
func claimTable(t *testing.T, h1, h2, h3 string) (*Game, *crafter) {
	t.Helper()
	g := NewGame(1, 0)
	c := newCrafter(t, g)
	c.hand(0, "123c 456c 789c 1z 2z 3z 4z 5d")
	c.hand(1, h1)
	c.hand(2, h2)
	c.hand(3, h3)
	discard := c.find(0, "5d")
	if _, err := g.Apply(0, Action{Type: ActionDiscard, TileID: discard.ID}); err != nil {
		t.Fatal(err)
	}
	if g.Phase() != PhaseClaim {
		t.Fatalf("expected a claim window, phase = %d", g.Phase())
	}
	return g, c
}

func chiAction(t *testing.T, g *Game, seat Seat) Action {
	t.Helper()
	for _, a := range g.LegalActions(seat) {
		if a.Type == ActionChi {
			return a
		}
	}
	t.Fatalf("seat %d has no chi option", seat)
	return Action{}
}

func TestPungClaim(t *testing.T) {
	g, _ := claimTable(t,
		"123b 456b 789b 5z 6z 7z 1z",
		"55d 123b 456b 789b 6z 7z",
		"123b 456b 789b 5z 6z 7z 2z")

	if got := g.Pending(); !slices.Equal(got, []Seat{2}) {
		t.Fatalf("pending = %v, want [2]", got)
	}
	acts := g.LegalActions(2)
	if !hasAction(acts, ActionPung) || !hasAction(acts, ActionPass) {
		t.Errorf("seat 2 should be offered pung and pass: %v", acts)
	}
	if hasAction(acts, ActionChi) || hasAction(acts, ActionHu) || hasAction(acts, ActionKong) {
		t.Errorf("seat 2 was offered more than pung/pass: %v", acts)
	}
	if g.LegalActions(1) != nil {
		t.Error("seat 1 has nothing to claim and must not be offered actions")
	}

	events, err := g.Apply(2, Action{Type: ActionPung})
	if err != nil {
		t.Fatal(err)
	}
	p := &g.players[2]
	if len(p.melds) != 1 || p.melds[0].Type != MeldPung || len(p.melds[0].Tiles) != 3 || p.melds[0].From != 0 {
		t.Errorf("meld wrong: %+v", p.melds)
	}
	if len(p.hand) != 11 {
		t.Errorf("hand = %d tiles, want 11", len(p.hand))
	}
	if g.Turn() != 2 || g.Phase() != PhaseDiscarding {
		t.Errorf("turn = %d phase = %d, want seat 2 discarding", g.Turn(), g.Phase())
	}
	if len(g.players[0].discards) != 0 {
		t.Error("claimed tile must leave the discard pile")
	}
	if events[0].Type != EventClaimed || events[0].Meld != MeldPung || events[0].Seat != 2 {
		t.Errorf("first event = %+v", events[0])
	}
	checkHandSizes(t, g)
}

func TestChiOnlyFromPreviousPlayer(t *testing.T) {
	g, _ := claimTable(t,
		"46d 123b 456b 789b 5z 6z",
		"123b 456b 789b 5z 6z 7z 3z",
		"46d 123b 456b 789b 5z 6z") // seat 3 holds the same tiles but is not next in turn

	if got := g.Pending(); !slices.Equal(got, []Seat{1}) {
		t.Fatalf("pending = %v, want [1]", got)
	}
	if _, err := g.Apply(3, Action{Type: ActionChi}); !errors.Is(err, ErrNotYourTurn) {
		t.Errorf("seat 3 chi: err = %v, want ErrNotYourTurn", err)
	}

	chi := chiAction(t, g, 1)
	bad := Action{Type: ActionChi, TileID: chi.TileID, TileID2: chi.TileID}
	if _, err := g.Apply(1, bad); !errors.Is(err, ErrIllegalAction) {
		t.Errorf("chi with the same tile twice: err = %v, want ErrIllegalAction", err)
	}

	if _, err := g.Apply(1, chi); err != nil {
		t.Fatal(err)
	}
	m := g.players[1].melds
	if len(m) != 1 || m[0].Type != MeldChi || len(m[0].Tiles) != 3 {
		t.Fatalf("meld wrong: %+v", m)
	}
	want := []Kind{KindDots1 + 3, KindDots1 + 4, KindDots1 + 5}
	for i, tile := range m[0].Tiles {
		if tile.Kind != want[i] {
			t.Errorf("chi tile %d = %v, want %v", i, tile.Kind, want[i])
		}
	}
	if g.Turn() != 1 || g.Phase() != PhaseDiscarding || len(g.players[1].hand) != 11 {
		t.Errorf("after chi: turn=%d phase=%d hand=%d", g.Turn(), g.Phase(), len(g.players[1].hand))
	}
	checkHandSizes(t, g)
}

func TestPungBeatsChi(t *testing.T) {
	hands := []string{
		"46d 123b 456b 789b 5z 6z",
		"55d 123b 456b 789b 6z 7z",
		"123b 456b 789b 5z 6z 7z 3z",
	}

	// Chi answers first, but seat 2 could still pung: the window stays open.
	g, _ := claimTable(t, hands[0], hands[1], hands[2])
	events, err := g.Apply(1, chiAction(t, g, 1))
	if err != nil || len(events) != 0 || g.Phase() != PhaseClaim {
		t.Fatalf("chi should wait for seat 2: events=%v err=%v phase=%d", events, err, g.Phase())
	}
	if _, err := g.Apply(2, Action{Type: ActionPung}); err != nil {
		t.Fatal(err)
	}
	if len(g.players[2].melds) != 1 || len(g.players[1].melds) != 0 || g.Turn() != 2 {
		t.Errorf("pung should win: melds seat1=%d seat2=%d turn=%d",
			len(g.players[1].melds), len(g.players[2].melds), g.Turn())
	}

	// Seat 2 passes, then seat 1's chi goes through.
	g, _ = claimTable(t, hands[0], hands[1], hands[2])
	if events, err := g.Apply(2, Action{Type: ActionPass}); err != nil || len(events) != 0 {
		t.Fatalf("pass should wait for seat 1: events=%v err=%v", events, err)
	}
	if _, err := g.Apply(1, chiAction(t, g, 1)); err != nil {
		t.Fatal(err)
	}
	if len(g.players[1].melds) != 1 || g.Turn() != 1 {
		t.Errorf("chi should win after the pung passed: melds=%d turn=%d", len(g.players[1].melds), g.Turn())
	}
}

func TestHuOnDiscardClosestWins(t *testing.T) {
	waiting := "123b 456b 789b 55z 46d" // waits on 5d
	pungger := "55d 123c 456c 789c 6z 7z"

	g, _ := claimTable(t, waiting, pungger, waiting)
	if got := g.Pending(); !slices.Equal(got, []Seat{1, 2, 3}) {
		t.Fatalf("pending = %v, want [1 2 3]", got)
	}
	if acts := g.LegalActions(1); acts[0].Type != ActionHu {
		t.Errorf("hu should be offered first: %v", acts)
	}
	// Pung and the far hu both have to wait for the closest hu.
	for _, step := range []struct {
		seat Seat
		act  Action
	}{{2, Action{Type: ActionPung}}, {3, Action{Type: ActionHu}}} {
		events, err := g.Apply(step.seat, step.act)
		if err != nil || len(events) != 0 || g.Phase() != PhaseClaim {
			t.Fatalf("seat %d should be left waiting: events=%v err=%v phase=%d", step.seat, events, err, g.Phase())
		}
	}
	events, err := g.Apply(1, Action{Type: ActionHu})
	if err != nil {
		t.Fatal(err)
	}
	res, ok := g.Result()
	if !ok || res.Draw || res.Winner != 1 || res.From != 0 || res.SelfDraw {
		t.Errorf("result = %+v ok=%v, want seat 1 winning on seat 0's discard", res, ok)
	}
	if events[len(events)-1].Type != EventGameEnd || !g.Ended() {
		t.Error("game should be over")
	}
	w := &g.players[1]
	if len(w.hand) != 14 || !IsWinning(CountsOf(w.hand), 0) {
		t.Errorf("winner's hand is not complete: %d tiles", len(w.hand))
	}
	if len(g.players[0].discards) != 0 {
		t.Error("winning tile must leave the discard pile")
	}

	// If the closest seat passes, the next one in turn order wins.
	g, _ = claimTable(t, waiting, pungger, waiting)
	for _, step := range []struct {
		seat Seat
		act  Action
	}{{1, Action{Type: ActionPass}}, {2, Action{Type: ActionPung}}} {
		if events, err := g.Apply(step.seat, step.act); err != nil || len(events) != 0 {
			t.Fatalf("seat %d should be left waiting: events=%v err=%v", step.seat, events, err)
		}
	}
	if _, err := g.Apply(3, Action{Type: ActionHu}); err != nil {
		t.Fatal(err)
	}
	if res, _ := g.Result(); res.Winner != 3 {
		t.Errorf("winner = %d, want 3", res.Winner)
	}
}

func TestKongFromDiscard(t *testing.T) {
	g, _ := claimTable(t,
		"123b 456b 789b 5z 6z 7z 1z",
		"555d 123b 456b 789b 6z",
		"123b 456b 789b 5z 6z 7z 2z")

	acts := g.LegalActions(2)
	if !hasAction(acts, ActionKong) || !hasAction(acts, ActionPung) {
		t.Fatalf("seat 2 should be offered pung and kong: %v", acts)
	}
	wallBefore := g.WallLeft()
	if _, err := g.Apply(2, Action{Type: ActionKong}); err != nil {
		t.Fatal(err)
	}
	p := &g.players[2]
	if len(p.melds) != 1 || p.melds[0].Type != MeldKong || len(p.melds[0].Tiles) != 4 {
		t.Errorf("meld wrong: %+v", p.melds)
	}
	if len(p.hand) != 11 {
		t.Errorf("hand = %d tiles, want 11 (10 + replacement)", len(p.hand))
	}
	if g.Turn() != 2 || g.Phase() != PhaseDiscarding {
		t.Errorf("turn = %d phase = %d, want seat 2 discarding", g.Turn(), g.Phase())
	}
	if g.WallLeft() >= wallBefore {
		t.Error("the replacement tile should come from the wall")
	}
	checkHandSizes(t, g)
}

func TestNoSelfHuRightAfterClaim(t *testing.T) {
	// Seat 2 could hu on 5d, but chooses pung. Its remaining tiles are
	// complete, yet no tile was drawn, so hu must not be offered.
	g, _ := claimTable(t,
		"123b 456b 789b 5z 6z 7z 1z",
		"55d 123b 456b 789b 11c",
		"123b 456b 789b 5z 6z 7z 2z")
	if !hasAction(g.LegalActions(2), ActionHu) {
		t.Fatal("seat 2 should be able to hu on the discard")
	}
	if _, err := g.Apply(2, Action{Type: ActionPung}); err != nil {
		t.Fatal(err)
	}
	if !IsWinning(CountsOf(g.players[2].hand), 1) {
		t.Fatal("test setup: remaining tiles should be complete")
	}
	if hasAction(g.LegalActions(2), ActionHu) {
		t.Error("hu offered right after a pung")
	}
	if _, err := g.Apply(2, Action{Type: ActionHu}); !errors.Is(err, ErrIllegalAction) {
		t.Errorf("hu after pung: err = %v, want ErrIllegalAction", err)
	}
}

func TestConcealedKong(t *testing.T) {
	g := NewGame(1, 0)
	c := newCrafter(t, g)
	c.hand(0, "1111d 234c 567c 89c 55z")

	var kong Action
	for _, a := range g.LegalActions(0) {
		if a.Type == ActionKong {
			kong = a
		}
	}
	if kong.Type != ActionKong {
		t.Fatalf("concealed kong not offered: %v", g.LegalActions(0))
	}
	events, err := g.Apply(0, kong)
	if err != nil {
		t.Fatal(err)
	}
	p := &g.players[0]
	if len(p.melds) != 1 || p.melds[0].Type != MeldConcealedKong || len(p.melds[0].Tiles) != 4 {
		t.Errorf("meld wrong: %+v", p.melds)
	}
	if len(p.hand) != 11 || g.Turn() != 0 || g.Phase() != PhaseDiscarding {
		t.Errorf("after kong: hand=%d turn=%d phase=%d", len(p.hand), g.Turn(), g.Phase())
	}
	if events[0].Type != EventClaimed || events[0].Meld != MeldConcealedKong {
		t.Fatalf("first event = %+v", events[0])
	}
	if got := events[0].ForSeat(1); got.Tiles != nil {
		t.Error("other players must not see a concealed kong")
	}
	if got := events[0].ForSeat(0); len(got.Tiles) != 4 {
		t.Error("the owner sees the kong")
	}
	checkHandSizes(t, g)
}

func TestAddedKong(t *testing.T) {
	g := NewGame(1, 0)
	c := newCrafter(t, g)
	c.hand(0, "5d 123c 456c 789c 1z")
	c.meld(0, MeldPung, "555d", 1)
	fifth := c.find(0, "5d")

	if _, err := g.Apply(0, Action{Type: ActionKong, TileID: c.find(0, "1z").ID}); !errors.Is(err, ErrIllegalAction) {
		t.Errorf("kong with a lone tile: err = %v, want ErrIllegalAction", err)
	}
	if _, err := g.Apply(0, Action{Type: ActionKong, TileID: fifth.ID}); err != nil {
		t.Fatal(err)
	}
	p := &g.players[0]
	if len(p.melds) != 1 || p.melds[0].Type != MeldKong || len(p.melds[0].Tiles) != 4 {
		t.Errorf("meld wrong: %+v", p.melds)
	}
	if len(p.hand) != 11 {
		t.Errorf("hand = %d tiles, want 11", len(p.hand))
	}
	checkHandSizes(t, g)
}

func TestNobodyCanClaim(t *testing.T) {
	g := NewGame(1, 0)
	c := newCrafter(t, g)
	c.hand(0, "123c 456c 789c 1z 2z 3z 4z 5d")
	c.hand(1, "123b 456b 789b 5z 6z 7z 1z")
	c.hand(2, "123b 456b 789b 5z 6z 7z 2z")
	c.hand(3, "123b 456b 789b 5z 6z 7z 3z")
	if _, err := g.Apply(0, Action{Type: ActionDiscard, TileID: c.find(0, "5d").ID}); err != nil {
		t.Fatal(err)
	}
	if g.Phase() != PhaseDiscarding || g.Turn() != 1 || len(g.players[1].hand) != 14 {
		t.Errorf("play should go straight to seat 1: phase=%d turn=%d hand=%d",
			g.Phase(), g.Turn(), len(g.players[1].hand))
	}
}

func TestExpireClaims(t *testing.T) {
	g, _ := claimTable(t,
		"123b 456b 789b 5z 6z 7z 1z",
		"55d 123b 456b 789b 6z 7z",
		"123b 456b 789b 5z 6z 7z 2z")
	events, err := g.ExpireClaims()
	if err != nil {
		t.Fatal(err)
	}
	if g.Phase() != PhaseDiscarding || g.Turn() != 1 || len(g.players[1].hand) != 14 {
		t.Errorf("after timeout: phase=%d turn=%d hand=%d", g.Phase(), g.Turn(), len(g.players[1].hand))
	}
	if len(events) == 0 {
		t.Error("expected events for the next draw")
	}
	if _, err := g.ExpireClaims(); !errors.Is(err, ErrIllegalAction) {
		t.Errorf("second ExpireClaims: err = %v, want ErrIllegalAction", err)
	}
}

func TestClaimWindowView(t *testing.T) {
	g, _ := claimTable(t,
		"123b 456b 789b 5z 6z 7z 1z",
		"55d 123b 456b 789b 6z 7z",
		"123b 456b 789b 5z 6z 7z 2z")
	v := g.ViewFor(2)
	if v.Phase != PhaseClaim || v.LastDiscard == nil || v.LastDiscard.Kind != KindDots1+4 || v.LastDiscarder != 0 {
		t.Errorf("claim view wrong: %+v", v)
	}
	if !hasAction(v.LegalActions, ActionPung) {
		t.Errorf("seat 2 view should list pung: %v", v.LegalActions)
	}
	if !slices.Equal(v.Pending, []Seat{2}) {
		t.Errorf("pending = %v, want [2]", v.Pending)
	}
	if v1 := g.ViewFor(1); v1.LegalActions != nil {
		t.Errorf("seat 1 should have no actions: %v", v1.LegalActions)
	}
}
