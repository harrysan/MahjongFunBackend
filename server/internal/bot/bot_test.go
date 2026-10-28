package bot

import (
	"testing"

	"mahjong/server/internal/engine"
)

// hand builds tiles of the given kinds with unique IDs.
func hand(kinds ...engine.Kind) []engine.Tile {
	used := map[engine.Kind]int{}
	out := make([]engine.Tile, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, engine.NewTile(uint8(int(k)*4+used[k])))
		used[k]++
	}
	return out
}

func discards(ts []engine.Tile) []engine.Action {
	acts := make([]engine.Action, 0, len(ts))
	for _, t := range ts {
		acts = append(acts, engine.Action{Type: engine.ActionDiscard, TileID: t.ID})
	}
	return acts
}

func dots(n int) engine.Kind { return engine.KindDots1 + engine.Kind(n-1) }

func TestDiscardsLoneHonorFirst(t *testing.T) {
	h := hand(dots(2), dots(3), dots(4), dots(5), dots(5), engine.KindEast)
	v := engine.PlayerView{Phase: engine.PhaseDiscarding, Hand: h, LegalActions: discards(h)}
	got := New(1).Choose(v)
	if got.Type != engine.ActionDiscard || got.TileID != h[5].ID {
		t.Errorf("chose %+v, want to discard the lone East (id %d)", got, h[5].ID)
	}
}

func TestKeepsPairsAndRuns(t *testing.T) {
	// 4-5-6 of dots, a pair of 9 bamboo, and one stray 1 of characters.
	h := hand(dots(4), dots(5), dots(6), engine.KindBamboo1+8, engine.KindBamboo1+8, engine.KindChars1)
	v := engine.PlayerView{Phase: engine.PhaseDiscarding, Hand: h, LegalActions: discards(h)}
	got := New(1).Choose(v)
	if got.TileID != h[5].ID {
		t.Errorf("chose tile %d, want the stray 1 of characters (id %d)", got.TileID, h[5].ID)
	}
}

func TestAlwaysHuFirst(t *testing.T) {
	v := engine.PlayerView{
		Phase: engine.PhaseClaim,
		LegalActions: []engine.Action{
			{Type: engine.ActionPung}, {Type: engine.ActionHu}, {Type: engine.ActionPass},
		},
	}
	if got := New(1).Choose(v); got.Type != engine.ActionHu {
		t.Errorf("chose %+v, want hu", got)
	}
}

func TestClaimPreferences(t *testing.T) {
	cases := []struct {
		name string
		acts []engine.Action
		want engine.ActionType
	}{
		{"pung over pass", []engine.Action{{Type: engine.ActionPung}, {Type: engine.ActionPass}}, engine.ActionPung},
		{"kong over pung", []engine.Action{{Type: engine.ActionPung}, {Type: engine.ActionKong}, {Type: engine.ActionPass}}, engine.ActionKong},
		{"pass when only pass", []engine.Action{{Type: engine.ActionPass}}, engine.ActionPass},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := engine.PlayerView{Phase: engine.PhaseClaim, LegalActions: tc.acts}
			if got := New(1).Choose(v); got.Type != tc.want {
				t.Errorf("chose %+v, want type %d", got, tc.want)
			}
		})
	}
}

func TestChiOnlySometimes(t *testing.T) {
	v := engine.PlayerView{
		Phase:        engine.PhaseClaim,
		LegalActions: []engine.Action{{Type: engine.ActionChi, TileID: 1, TileID2: 2}, {Type: engine.ActionPass}},
	}
	b := New(1)
	chis := 0
	for i := 0; i < 2000; i++ {
		if b.Choose(v).Type == engine.ActionChi {
			chis++
		}
	}
	if chis < 300 || chis > 700 {
		t.Errorf("chi taken %d/2000 times, expected roughly 25%%", chis)
	}
}

func TestDeclaresKongOnOwnTurn(t *testing.T) {
	h := hand(dots(1), dots(1), dots(1), dots(1), dots(5))
	acts := append([]engine.Action{{Type: engine.ActionKong, TileID: h[0].ID}}, discards(h)...)
	v := engine.PlayerView{Phase: engine.PhaseDiscarding, Hand: h, LegalActions: acts}
	if got := New(1).Choose(v); got.Type != engine.ActionKong {
		t.Errorf("chose %+v, want kong", got)
	}
}

func TestNoActionsGivesZeroAction(t *testing.T) {
	if got := New(1).Choose(engine.PlayerView{}); got != (engine.Action{}) {
		t.Errorf("chose %+v, want the zero Action", got)
	}
}

// Four bots play complete games against each other through the public engine API.
func TestBotsPlayFullGames(t *testing.T) {
	var bots [engine.NumSeats]*Bot
	draws, selfWins, discardWins := 0, 0, 0
	for seed := int64(1); seed <= 500; seed++ {
		for s := range bots {
			bots[s] = New(seed*10 + int64(s))
		}
		g := engine.NewGame(seed, engine.Seat(seed%engine.NumSeats))
		for steps := 0; !g.Ended(); steps++ {
			if steps > 3000 {
				t.Fatalf("seed %d: game did not finish", seed)
			}
			seat := g.Pending()[0]
			act := bots[seat].Choose(g.ViewFor(seat))
			if _, err := g.Apply(seat, act); err != nil {
				t.Fatalf("seed %d: bot at seat %d chose %+v, engine said: %v", seed, seat, act, err)
			}
		}
		res, ok := g.Result()
		if !ok {
			t.Fatalf("seed %d: game ended without a result", seed)
		}
		switch {
		case res.Draw:
			draws++
		case res.SelfDraw:
			selfWins++
		default:
			discardWins++
		}
	}
	t.Logf("500 bot games: %d draws, %d self-draw wins, %d wins on a discard", draws, selfWins, discardWins)
}
