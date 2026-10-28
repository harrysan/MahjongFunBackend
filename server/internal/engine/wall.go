package engine

import "math/rand"

// DeadWallSize is how many tiles at the end of the wall are never drawn.
// The spec says the round is drawn when the wall runs out, so it is 0 for now.
const DeadWallSize = 0

// Wall is the shuffled stack of tiles. Normal draws take from the front;
// replacement draws (after a bonus tile or a kong) take from the back.
type Wall struct {
	tiles       []Tile
	front, back int // next index to draw from the front / from the back
}

// NewWall builds and shuffles all 144 tiles. The same rng state gives the same wall.
func NewWall(rng *rand.Rand) *Wall {
	tiles := make([]Tile, NumTiles)
	for i := range tiles {
		tiles[i] = NewTile(uint8(i))
	}
	rng.Shuffle(len(tiles), func(i, j int) { tiles[i], tiles[j] = tiles[j], tiles[i] })
	return &Wall{tiles: tiles, front: 0, back: len(tiles) - 1}
}

// Remaining is how many tiles can still be drawn.
func (w *Wall) Remaining() int {
	r := w.back - w.front + 1 - DeadWallSize
	if r < 0 {
		return 0
	}
	return r
}

// DrawFront takes the next tile from the front. ok is false when the wall is exhausted.
func (w *Wall) DrawFront() (Tile, bool) {
	if w.Remaining() <= 0 {
		return Tile{}, false
	}
	t := w.tiles[w.front]
	w.front++
	return t, true
}

// DrawBack takes a replacement tile from the back. ok is false when the wall is exhausted.
func (w *Wall) DrawBack() (Tile, bool) {
	if w.Remaining() <= 0 {
		return Tile{}, false
	}
	t := w.tiles[w.back]
	w.back--
	return t, true
}
