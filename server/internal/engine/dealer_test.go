package engine

import (
	"math/rand"
	"testing"
)

func TestNextDealer(t *testing.T) {
	cases := []struct {
		winner Seat
		dice   [2]int
		want   Seat
	}{
		{2, [2]int{2, 3}, 2}, // total 5: counts 2,3,0,1,2 -> lands on winner
		{2, [2]int{3, 3}, 3}, // total 6
		{0, [2]int{1, 1}, 1}, // total 2: winner is 1, next seat is 2
		{0, [2]int{6, 6}, 3}, // total 12
		{3, [2]int{1, 1}, 0}, // wraps around
		{1, [2]int{4, 4}, 0}, // total 8
	}
	for _, tc := range cases {
		if got := NextDealer(tc.winner, tc.dice); got != tc.want {
			t.Errorf("NextDealer(%d, %v) = %d, want %d", tc.winner, tc.dice, got, tc.want)
		}
	}
}

func TestNextDealerAlwaysValid(t *testing.T) {
	for w := Seat(0); w < NumSeats; w++ {
		for a := 1; a <= 6; a++ {
			for b := 1; b <= 6; b++ {
				if d := NextDealer(w, [2]int{a, b}); !d.Valid() {
					t.Fatalf("NextDealer(%d, [%d %d]) = %d is not a seat", w, a, b, d)
				}
			}
		}
	}
}

func TestRollDiceRange(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	seen := map[int]bool{}
	for i := 0; i < 1000; i++ {
		d := RollDice(rng)
		for _, v := range d {
			if v < 1 || v > 6 {
				t.Fatalf("die value %d out of range", v)
			}
			seen[v] = true
		}
	}
	if len(seen) != 6 {
		t.Errorf("expected all six faces in 1000 rolls, saw %d", len(seen))
	}
}
