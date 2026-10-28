package engine

import (
	"math/rand"
	"testing"
)

func wallIDs(w *Wall) []uint8 {
	ids := make([]uint8, len(w.tiles))
	for i, t := range w.tiles {
		ids[i] = t.ID
	}
	return ids
}

func TestWallDeterministic(t *testing.T) {
	a := wallIDs(NewWall(rand.New(rand.NewSource(42))))
	b := wallIDs(NewWall(rand.New(rand.NewSource(42))))
	c := wallIDs(NewWall(rand.New(rand.NewSource(43))))
	same := func(x, y []uint8) bool {
		for i := range x {
			if x[i] != y[i] {
				return false
			}
		}
		return true
	}
	if !same(a, b) {
		t.Error("same seed produced different walls")
	}
	if same(a, c) {
		t.Error("different seeds produced the same wall")
	}
}

func TestWallDrawAll(t *testing.T) {
	w := NewWall(rand.New(rand.NewSource(1)))
	seen := map[uint8]bool{}
	for i := 0; i < NumTiles; i++ {
		var tile Tile
		var ok bool
		if i%2 == 0 {
			tile, ok = w.DrawFront()
		} else {
			tile, ok = w.DrawBack()
		}
		if !ok {
			t.Fatalf("draw %d failed early", i)
		}
		if seen[tile.ID] {
			t.Fatalf("tile %d drawn twice", tile.ID)
		}
		seen[tile.ID] = true
	}
	if w.Remaining() != 0 {
		t.Errorf("remaining = %d, want 0", w.Remaining())
	}
	if _, ok := w.DrawFront(); ok {
		t.Error("DrawFront should fail on empty wall")
	}
	if _, ok := w.DrawBack(); ok {
		t.Error("DrawBack should fail on empty wall")
	}
}
