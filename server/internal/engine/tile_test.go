package engine

import "testing"

func TestAllTiles(t *testing.T) {
	ids := map[uint8]bool{}
	perKind := map[Kind]int{}
	for i := 0; i < NumTiles; i++ {
		tile := NewTile(uint8(i))
		if ids[tile.ID] {
			t.Fatalf("duplicate id %d", tile.ID)
		}
		ids[tile.ID] = true
		perKind[tile.Kind]++
	}
	for k := Kind(0); k < NumPlayKinds; k++ {
		if perKind[k] != 4 {
			t.Errorf("kind %v has %d tiles, want 4", k, perKind[k])
		}
	}
	for k := Kind(NumPlayKinds); k < NumKinds; k++ {
		if perKind[k] != 1 {
			t.Errorf("bonus kind %v has %d tiles, want 1", k, perKind[k])
		}
	}
}

func TestKindHelpers(t *testing.T) {
	if k := KindDots1; !k.IsSuited() || k.Number() != 1 || k.Suit() != 0 {
		t.Errorf("Dots1 helpers wrong")
	}
	if k := KindBamboo1 + 8; k.Number() != 9 || k.Suit() != 1 {
		t.Errorf("Bamboo9 helpers wrong")
	}
	if k := KindChars1 + 4; k.Number() != 5 || k.Suit() != 2 {
		t.Errorf("Chars5 helpers wrong")
	}
	if !KindEast.IsHonor() || KindEast.IsSuited() || KindEast.IsBonus() {
		t.Errorf("East helpers wrong")
	}
	if !KindFlower1.IsBonus() || !(KindSeason1+3).IsBonus() || KindWhite.IsBonus() {
		t.Errorf("bonus helpers wrong")
	}
	if KindBamboo1.String() != "B1" || KindRed.String() != "Red" || KindSeason1.String() != "Season1" {
		t.Errorf("String() wrong: %v %v %v", KindBamboo1, KindRed, KindSeason1)
	}
}
