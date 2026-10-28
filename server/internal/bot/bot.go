// Package bot holds the computer players. A bot only sees what a human at
// the table would see (a PlayerView) and only picks from the legal actions
// the engine offers, so it can neither cheat nor make an illegal move.
package bot

import (
	"math/rand"

	"mahjong/server/internal/engine"
)

// chiChance is how often a bot takes a chi when one is offered.
const chiChance = 0.25

// Bot is one computer player.
type Bot struct{ rng *rand.Rand }

// New creates a bot; the same seed gives the same choices.
func New(seed int64) *Bot { return &Bot{rng: rand.New(rand.NewSource(seed))} }

// Choose picks an action for the seat that v belongs to. When v offers no
// legal action (not this seat's turn, or the game is over) it returns the zero Action.
//
// Strategy (v1):
//   - always hu when possible
//   - claim window: kong, then pung, chi only now and then, otherwise pass
//   - own turn: declare a kong if offered, otherwise discard the least useful tile
func (b *Bot) Choose(v engine.PlayerView) engine.Action {
	acts := v.LegalActions
	if len(acts) == 0 {
		return engine.Action{}
	}
	if a, ok := first(acts, engine.ActionHu); ok {
		return a
	}
	if v.Phase == engine.PhaseClaim {
		return b.chooseClaim(acts)
	}
	return b.chooseDiscard(v, acts)
}

func (b *Bot) chooseClaim(acts []engine.Action) engine.Action {
	if a, ok := first(acts, engine.ActionKong); ok {
		return a
	}
	if a, ok := first(acts, engine.ActionPung); ok {
		return a
	}
	if a, ok := first(acts, engine.ActionChi); ok && b.rng.Float64() < chiChance {
		return a
	}
	return engine.Action{Type: engine.ActionPass}
}

func (b *Bot) chooseDiscard(v engine.PlayerView, acts []engine.Action) engine.Action {
	if a, ok := first(acts, engine.ActionKong); ok {
		return a
	}
	counts := engine.CountsOf(v.Hand)
	byID := make(map[uint8]engine.Tile, len(v.Hand))
	for _, t := range v.Hand {
		byID[t.ID] = t
	}

	best := -1
	var candidates []engine.Action
	for _, a := range acts {
		if a.Type != engine.ActionDiscard {
			continue
		}
		t, ok := byID[a.TileID]
		if !ok {
			continue
		}
		s := tileScore(counts, t.Kind)
		switch {
		case best == -1 || s < best:
			best = s
			candidates = append(candidates[:0], a)
		case s == best:
			candidates = append(candidates, a)
		}
	}
	if len(candidates) == 0 {
		return acts[0]
	}
	return candidates[b.rng.Intn(len(candidates))]
}

// tileScore says how useful a tile is to the hand; the bot discards the lowest.
// Pairs and triples score high, tiles with neighbours in the same suit score
// medium, lone honors and lone terminals score lowest.
func tileScore(c engine.Counts, k engine.Kind) int {
	s := 0
	switch n := c[k]; {
	case n >= 3:
		s += 6
	case n == 2:
		s += 4
	}
	if k.IsSuited() {
		num := k.Number()
		if num == 1 || num == 9 {
			s++
		} else {
			s += 2
		}
		if num >= 2 && c[k-1] > 0 {
			s += 2
		}
		if num <= 8 && c[k+1] > 0 {
			s += 2
		}
		if num >= 3 && c[k-2] > 0 {
			s++
		}
		if num <= 7 && c[k+2] > 0 {
			s++
		}
	}
	return s
}

func first(acts []engine.Action, typ engine.ActionType) (engine.Action, bool) {
	for _, a := range acts {
		if a.Type == typ {
			return a, true
		}
	}
	return engine.Action{}, false
}
