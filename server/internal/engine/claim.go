package engine

import "sort"

// claimOpts is what one seat may do with the tile that was just discarded.
type claimOpts struct{ hu, pung, kong, chi bool }

func (o claimOpts) has() bool { return o.hu || o.pung || o.kong || o.chi }

// claimWindow tracks the answers to one discard.
type claimWindow struct {
	discarder Seat
	tile      Tile
	opts      [NumSeats]claimOpts
	responded [NumSeats]bool
	claimed   [NumSeats]bool   // answered with a claim (not a pass)
	choice    [NumSeats]Action // the claim, valid when claimed[seat]
}

// openClaimWindow looks at every other seat. g.claim stays nil when nobody can claim.
func (g *Game) openClaimWindow(discarder Seat, t Tile) {
	w := &claimWindow{discarder: discarder, tile: t}
	found := false
	for s := Seat(0); s < NumSeats; s++ {
		if s == discarder {
			continue
		}
		o := g.claimOptionsFor(s, discarder, t)
		w.opts[s] = o
		if o.has() {
			found = true
		}
	}
	if found {
		g.claim = w
	} else {
		g.claim = nil
	}
}

func (g *Game) claimOptionsFor(s, discarder Seat, t Tile) claimOpts {
	p := &g.players[s]
	c := CountsOf(p.hand)
	k := t.Kind
	var o claimOpts
	if c[k] >= 2 {
		o.pung = true
	}
	if c[k] >= 3 {
		o.kong = true
	}
	c[k]++
	if IsWinning(c, len(p.melds)) {
		o.hu = true
	}
	if s == discarder.Next() && len(g.chiOptions(s, t)) > 0 {
		o.chi = true
	}
	return o
}

// chiOptions lists the pairs of hand tiles that make a sequence with t.
func (g *Game) chiOptions(s Seat, t Tile) [][2]Tile {
	k := t.Kind
	if !k.IsSuited() {
		return nil
	}
	n := k.Number()
	var pairs [][2]Kind
	if n >= 3 {
		pairs = append(pairs, [2]Kind{k - 2, k - 1})
	}
	if n >= 2 && n <= 8 {
		pairs = append(pairs, [2]Kind{k - 1, k + 1})
	}
	if n <= 7 {
		pairs = append(pairs, [2]Kind{k + 1, k + 2})
	}
	hand := g.players[s].hand
	var out [][2]Tile
	for _, pr := range pairs {
		a, okA := firstOfKind(hand, pr[0])
		b, okB := firstOfKind(hand, pr[1])
		if okA && okB {
			out = append(out, [2]Tile{a, b})
		}
	}
	return out
}

// chiValid checks that the two named hand tiles and the discard form a sequence.
func (g *Game) chiValid(seat Seat, discard Tile, id1, id2 uint8) bool {
	if id1 == id2 {
		return false
	}
	hand := g.players[seat].hand
	a, okA := findByID(hand, id1)
	b, okB := findByID(hand, id2)
	if !okA || !okB {
		return false
	}
	ks := []int{int(discard.Kind), int(a.Kind), int(b.Kind)}
	sort.Ints(ks)
	for _, k := range ks {
		if !Kind(k).IsSuited() {
			return false
		}
	}
	return ks[1] == ks[0]+1 && ks[2] == ks[1]+1 && Kind(ks[0]).Suit() == Kind(ks[2]).Suit()
}

// Priority: hu (3) > pung/kong (2) > chi (1); ties go to the seat closest after the discarder.
func tierOf(a ActionType) int {
	switch a {
	case ActionHu:
		return 3
	case ActionPung, ActionKong:
		return 2
	case ActionChi:
		return 1
	}
	return 0
}

func (w *claimWindow) rank(s Seat, tier int) int {
	dist := (int(s) - int(w.discarder) + NumSeats) % NumSeats // 1..3
	return tier*10 + (NumSeats - dist)
}

// potential is the best rank seat s could still reach with its options.
func (w *claimWindow) potential(s Seat) int {
	o := w.opts[s]
	switch {
	case o.hu:
		return w.rank(s, 3)
	case o.pung || o.kong:
		return w.rank(s, 2)
	case o.chi:
		return w.rank(s, 1)
	}
	return 0
}

func (g *Game) applyClaimResponse(seat Seat, a Action) ([]Event, error) {
	w := g.claim
	if w == nil || seat == w.discarder || !w.opts[seat].has() || w.responded[seat] {
		return nil, ErrNotYourTurn
	}
	o := w.opts[seat]
	switch a.Type {
	case ActionPass:
	case ActionHu:
		if !o.hu {
			return nil, ErrIllegalAction
		}
	case ActionPung:
		if !o.pung {
			return nil, ErrIllegalAction
		}
	case ActionKong:
		if !o.kong {
			return nil, ErrIllegalAction
		}
	case ActionChi:
		if !o.chi || !g.chiValid(seat, w.tile, a.TileID, a.TileID2) {
			return nil, ErrIllegalAction
		}
	default:
		return nil, ErrIllegalAction
	}
	w.responded[seat] = true
	if a.Type != ActionPass {
		w.claimed[seat] = true
		w.choice[seat] = a
	}
	return g.resolveIfDone()
}

// ExpireClaims is called by the room when the claim timer runs out:
// every seat that has not answered counts as a pass.
func (g *Game) ExpireClaims() ([]Event, error) {
	if g.phase != PhaseClaim || g.claim == nil {
		return nil, ErrIllegalAction
	}
	for s := Seat(0); s < NumSeats; s++ {
		if g.claim.opts[s].has() {
			g.claim.responded[s] = true
		}
	}
	return g.resolveIfDone()
}

// resolveIfDone settles the window as soon as no seat that is still undecided
// could outrank the best claim made so far. It returns no events while waiting.
func (g *Game) resolveIfDone() ([]Event, error) {
	w := g.claim
	best, bestSeat := 0, Seat(-1)
	for s := Seat(0); s < NumSeats; s++ {
		if w.claimed[s] {
			if r := w.rank(s, tierOf(w.choice[s].Type)); r > best {
				best, bestSeat = r, s
			}
		}
	}
	for s := Seat(0); s < NumSeats; s++ {
		if s == w.discarder || w.responded[s] {
			continue
		}
		if w.potential(s) > best {
			return nil, nil // someone could still outrank the current best
		}
	}

	var events []Event
	if bestSeat < 0 {
		g.claim = nil
		g.nextTurn(&events)
		return events, nil
	}
	return g.executeClaim(bestSeat, w.choice[bestSeat], w), nil
}

func (g *Game) executeClaim(seat Seat, a Action, w *claimWindow) []Event {
	p := &g.players[seat]
	d := &g.players[w.discarder]
	tile := d.discards[len(d.discards)-1] // the claimed tile leaves the discard pile
	d.discards = d.discards[:len(d.discards)-1]
	g.claim = nil

	var events []Event
	switch a.Type {
	case ActionHu:
		p.hand = append(p.hand, tile)
		g.phase = PhaseEnded
		g.result = &Result{Winner: seat, From: w.discarder, WinTile: tile}
		events = append(events, Event{Type: EventGameEnd, Seat: seat})

	case ActionPung:
		taken := removeByKind(p, tile.Kind, 2)
		g.addMeld(seat, MeldPung, append([]Tile{tile}, taken...), w.discarder, &events)
		g.turn, g.phase, g.canSelfHu = seat, PhaseDiscarding, false

	case ActionChi:
		x, _ := removeByID(p, a.TileID)
		y, _ := removeByID(p, a.TileID2)
		ts := []Tile{tile, x, y}
		sort.Slice(ts, func(i, j int) bool { return ts[i].Kind < ts[j].Kind })
		g.addMeld(seat, MeldChi, ts, w.discarder, &events)
		g.turn, g.phase, g.canSelfHu = seat, PhaseDiscarding, false

	case ActionKong:
		taken := removeByKind(p, tile.Kind, 3)
		g.addMeld(seat, MeldKong, append([]Tile{tile}, taken...), w.discarder, &events)
		g.turn, g.phase = seat, PhaseDiscarding
		if !g.draw(seat, true, &events) { // replacement tile from the back of the wall
			g.endDraw(&events)
		}
	}
	return events
}

func (g *Game) addMeld(seat Seat, typ MeldType, tiles []Tile, from Seat, events *[]Event) {
	p := &g.players[seat]
	p.melds = append(p.melds, Meld{Type: typ, Tiles: tiles, From: from})
	*events = append(*events, Event{Type: EventClaimed, Seat: seat, Meld: typ, Tiles: cloneTiles(tiles)})
}

// selfKongOptions lists one tile ID per kong the current player could declare:
// four of a kind in hand (concealed kong), or a 4th tile for an existing pung (added kong).
func (g *Game) selfKongOptions(seat Seat) []uint8 {
	p := &g.players[seat]
	c := CountsOf(p.hand)
	seen := map[Kind]bool{}
	var ids []uint8
	for _, t := range p.hand {
		if t.Kind.IsBonus() || seen[t.Kind] {
			continue
		}
		if c[t.Kind] == 4 {
			ids = append(ids, t.ID)
			seen[t.Kind] = true
			continue
		}
		for _, m := range p.melds {
			if m.Type == MeldPung && m.Tiles[0].Kind == t.Kind {
				ids = append(ids, t.ID)
				seen[t.Kind] = true
				break
			}
		}
	}
	return ids
}

func (g *Game) applyKongSelf(seat Seat, id uint8) ([]Event, error) {
	p := &g.players[seat]
	target, ok := findByID(p.hand, id)
	if !ok {
		return nil, ErrTileNotInHand
	}
	kind := target.Kind
	n := 0
	for _, t := range p.hand {
		if t.Kind == kind {
			n++
		}
	}

	var events []Event
	if n == 4 {
		taken := removeByKind(p, kind, 4)
		p.melds = append(p.melds, Meld{Type: MeldConcealedKong, Tiles: taken, From: seat})
		events = append(events, Event{Type: EventClaimed, Seat: seat, Meld: MeldConcealedKong, Tiles: cloneTiles(taken)})
	} else {
		mi := -1
		for i, m := range p.melds {
			if m.Type == MeldPung && m.Tiles[0].Kind == kind {
				mi = i
				break
			}
		}
		if mi < 0 {
			return nil, ErrIllegalAction
		}
		t, _ := removeByID(p, id)
		p.melds[mi].Tiles = append(p.melds[mi].Tiles, t)
		p.melds[mi].Type = MeldKong
		events = append(events, Event{Type: EventClaimed, Seat: seat, Meld: MeldKong, Tiles: []Tile{t}})
	}
	if !g.draw(seat, true, &events) { // replacement tile from the back of the wall
		g.endDraw(&events)
	}
	return events, nil
}

func firstOfKind(hand []Tile, k Kind) (Tile, bool) {
	for _, t := range hand {
		if t.Kind == k {
			return t, true
		}
	}
	return Tile{}, false
}

func findByID(hand []Tile, id uint8) (Tile, bool) {
	for _, t := range hand {
		if t.ID == id {
			return t, true
		}
	}
	return Tile{}, false
}

func removeByID(p *player, id uint8) (Tile, bool) {
	for i, t := range p.hand {
		if t.ID == id {
			p.hand = append(p.hand[:i], p.hand[i+1:]...)
			return t, true
		}
	}
	return Tile{}, false
}

// removeByKind takes up to n tiles of kind k out of the hand and returns them.
func removeByKind(p *player, k Kind, n int) []Tile {
	var taken []Tile
	kept := p.hand[:0]
	for _, t := range p.hand {
		if t.Kind == k && len(taken) < n {
			taken = append(taken, t)
		} else {
			kept = append(kept, t)
		}
	}
	p.hand = kept
	return taken
}
