package engine

import "testing"

// parseHand reads a compact hand notation into counts.
// Digits followed by a suit letter: d = dots, b = bamboo, c = characters,
// z = honors (1-4 winds East..North, 5-7 dragons Red, Green, White).
// Example: "123d 456b 55z". Spaces are ignored.
func parseHand(t *testing.T, s string) Counts {
	t.Helper()
	var c Counts
	var digits []int
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digits = append(digits, int(r-'0'))
		case r == 'd' || r == 'b' || r == 'c' || r == 'z':
			base := map[rune]int{'d': 0, 'b': 9, 'c': 18, 'z': 27}[r]
			for _, n := range digits {
				if n < 1 || n > 9 || (r == 'z' && n > 7) {
					t.Fatalf("bad number %d for suit %c in %q", n, r, s)
				}
				c[base+n-1]++
			}
			digits = nil
		case r == ' ':
		default:
			t.Fatalf("unexpected %q in hand %q", r, s)
		}
	}
	if len(digits) != 0 {
		t.Fatalf("digits without suit in %q", s)
	}
	return c
}

// tilesFor expands a hand string into concrete tiles with unique IDs.
func tilesFor(t *testing.T, s string) []Tile {
	t.Helper()
	c := parseHand(t, s)
	var out []Tile
	for k := 0; k < NumPlayKinds; k++ {
		for i := 0; i < int(c[k]); i++ {
			out = append(out, NewTile(uint8(k*4+i)))
		}
	}
	return out
}

// checkConservation verifies every one of the 144 tiles exists exactly once
// across hands, bonus areas, discards, melds and the remaining wall.
func checkConservation(t *testing.T, g *Game) {
	t.Helper()
	seen := map[uint8]bool{}
	add := func(ts []Tile) {
		for _, x := range ts {
			if seen[x.ID] {
				t.Fatalf("tile %d appears twice", x.ID)
			}
			seen[x.ID] = true
		}
	}
	for s := range g.players {
		p := &g.players[s]
		add(p.hand)
		add(p.bonus)
		add(p.discards)
		for _, m := range p.melds {
			add(m.Tiles)
		}
	}
	add(g.wall.tiles[g.wall.front : g.wall.back+1])
	if len(seen) != NumTiles {
		t.Fatalf("expected %d distinct tiles, got %d", NumTiles, len(seen))
	}
}

// checkHandSizes verifies each seat holds 13 - 3*melds concealed tiles,
// plus one for the player who has to discard.
func checkHandSizes(t *testing.T, g *Game) {
	t.Helper()
	if g.Ended() {
		return
	}
	for s := Seat(0); s < NumSeats; s++ {
		p := &g.players[s]
		want := 13 - 3*len(p.melds)
		if g.Phase() == PhaseDiscarding && s == g.Turn() {
			want++
		}
		if len(p.hand) != want {
			t.Fatalf("seat %d holds %d tiles, want %d (phase %d, turn %d, melds %d)",
				s, len(p.hand), want, g.Phase(), g.Turn(), len(p.melds))
		}
	}
}

func hasAction(acts []Action, typ ActionType) bool {
	for _, a := range acts {
		if a.Type == typ {
			return true
		}
	}
	return false
}

// discardAndSettle discards a tile and, if a claim window opens, lets every
// seat pass so play continues. It returns all events produced.
func discardAndSettle(t *testing.T, g *Game, seat Seat, id uint8) []Event {
	t.Helper()
	events, err := g.Apply(seat, Action{Type: ActionDiscard, TileID: id})
	if err != nil {
		t.Fatal(err)
	}
	if g.Phase() == PhaseClaim {
		more, err := g.ExpireClaims()
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, more...)
	}
	return events
}

// crafter builds hands with chosen tiles while keeping tile IDs unique.
type crafter struct {
	t    *testing.T
	g    *Game
	used map[uint8]bool
}

func newCrafter(t *testing.T, g *Game) *crafter {
	return &crafter{t: t, g: g, used: map[uint8]bool{}}
}

func (c *crafter) tiles(s string) []Tile {
	c.t.Helper()
	counts := parseHand(c.t, s)
	var out []Tile
	for k := 0; k < NumPlayKinds; k++ {
		for i := 0; i < int(counts[k]); i++ {
			placed := false
			for j := 0; j < 4; j++ {
				id := uint8(k*4 + j)
				if !c.used[id] {
					c.used[id] = true
					out = append(out, NewTile(id))
					placed = true
					break
				}
			}
			if !placed {
				c.t.Fatalf("no copies of %v left while building %q", Kind(k), s)
			}
		}
	}
	return out
}

// hand replaces seat's concealed tiles and clears its melds, bonus and discards.
func (c *crafter) hand(seat Seat, s string) {
	c.t.Helper()
	p := &c.g.players[seat]
	p.hand = c.tiles(s)
	p.melds, p.bonus, p.discards = nil, nil, nil
}

// meld adds a declared meld for seat (call after hand).
func (c *crafter) meld(seat Seat, typ MeldType, s string, from Seat) {
	c.t.Helper()
	p := &c.g.players[seat]
	p.melds = append(p.melds, Meld{Type: typ, Tiles: c.tiles(s), From: from})
}

// find returns the first tile in seat's hand with the kind written in s (e.g. "5d").
func (c *crafter) find(seat Seat, s string) Tile {
	c.t.Helper()
	counts := parseHand(c.t, s)
	for k := 0; k < NumPlayKinds; k++ {
		if counts[k] == 0 {
			continue
		}
		for _, tile := range c.g.players[seat].hand {
			if tile.Kind == Kind(k) {
				return tile
			}
		}
	}
	c.t.Fatalf("seat %d holds no %q", seat, s)
	return Tile{}
}
