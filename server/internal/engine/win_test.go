package engine

import (
	"slices"
	"testing"
)

func TestIsWinning(t *testing.T) {
	cases := []struct {
		name  string
		hand  string
		melds int
		want  bool
	}{
		{"four chows and a pair", "123d 456d 789d 123b 55c", 0, true},
		{"pungs of suits, winds and dragons", "111d 222b 333c 444z 55z", 0, true},
		{"nine gates plus a 5", "11123455678999d", 0, true},
		{"pair is not a pair", "123d 456d 789d 123b 56c", 0, false},
		{"seven pairs is not supported", "11d 33d 55d 22b 44b 66c 77c", 0, false},
		{"1-8-9 does not wrap around", "123d 456d 789d 55b 189b", 0, false},
		{"dragons are not a sequence", "123d 456d 789d 567z 11b", 0, false},
		{"one meld declared", "123d 456d 789d 55b", 1, true},
		{"two melds declared", "123d 456d 55b", 2, true},
		{"four melds declared", "55b", 4, true},
		{"13 tiles is not complete", "123d 456d 789d 123b 5c", 0, false},
		{"quad in hand splits into pung + chow", "1111d 23d 456d 789d 55b", 0, true},
		{"needs pung-first split", "22233344d 123b", 1, true},
		{"too many melds", "55b", 5, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsWinning(parseHand(t, tc.hand), tc.melds)
			if got != tc.want {
				t.Errorf("IsWinning(%q, melds=%d) = %v, want %v", tc.hand, tc.melds, got, tc.want)
			}
		})
	}
}

func TestIsWinningDoesNotMutate(t *testing.T) {
	c := parseHand(t, "123d 456d 789d 123b 55c")
	before := c
	IsWinning(c, 0)
	if c != before {
		t.Error("IsWinning changed its input")
	}
}

func TestWaits(t *testing.T) {
	cases := []struct {
		name string
		hand string
		want []Kind
	}{
		{"single wait", "123d 456d 789d 123b 5c", []Kind{KindChars1 + 4}},
		{"two-sided wait", "123d 456d 789d 55b 34c", []Kind{KindChars1 + 1, KindChars1 + 4}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Waits(parseHand(t, tc.hand), 0)
			if !slices.Equal(got, tc.want) {
				t.Errorf("Waits(%q) = %v, want %v", tc.hand, got, tc.want)
			}
		})
	}
}

func TestCountsOfSkipsBonus(t *testing.T) {
	tiles := []Tile{NewTile(0), NewTile(1), NewTile(136), NewTile(143)}
	c := CountsOf(tiles)
	if c[0] != 2 {
		t.Errorf("c[0] = %d, want 2", c[0])
	}
	total := 0
	for _, n := range c {
		total += int(n)
	}
	if total != 2 {
		t.Errorf("bonus tiles were counted, total = %d", total)
	}
}
