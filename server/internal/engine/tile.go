package engine

import "fmt"

// Kind identifies the type of a tile (all four copies of "5 Bamboo" share a Kind).
//
//	0-8   Dots 1-9
//	9-17  Bamboo 1-9
//	18-26 Characters 1-9
//	27-30 Winds: East, South, West, North
//	31-33 Dragons: Red, Green, White
//	34-37 Flowers 1-4   (bonus)
//	38-41 Seasons 1-4   (bonus)
type Kind uint8

const (
	KindDots1    Kind = 0
	KindBamboo1  Kind = 9
	KindChars1   Kind = 18
	KindEast     Kind = 27
	KindSouth    Kind = 28
	KindWest     Kind = 29
	KindNorth    Kind = 30
	KindRed      Kind = 31
	KindGreen    Kind = 32
	KindWhite    Kind = 33
	KindFlower1  Kind = 34
	KindSeason1  Kind = 38
	NumSuited         = 27  // dots, bamboo, characters
	NumPlayKinds      = 34  // kinds that can be part of a set or pair
	NumKinds          = 42  // play kinds + bonus kinds
	NumTiles          = 144 // 34*4 + 8 bonus tiles
)

// IsBonus reports whether k is a flower or season tile.
func (k Kind) IsBonus() bool { return k >= NumPlayKinds && k < NumKinds }

// IsSuited reports whether k is a dots/bamboo/characters tile.
func (k Kind) IsSuited() bool { return k < NumSuited }

// IsHonor reports whether k is a wind or dragon tile.
func (k Kind) IsHonor() bool { return k >= NumSuited && k < NumPlayKinds }

// Suit returns 0 (dots), 1 (bamboo) or 2 (characters). Only valid for suited kinds.
func (k Kind) Suit() int { return int(k) / 9 }

// Number returns 1-9 for suited kinds. Only valid for suited kinds.
func (k Kind) Number() int { return int(k)%9 + 1 }

// String returns a short readable name, e.g. "D5", "B1", "East", "Flower2".
func (k Kind) String() string {
	switch {
	case k.IsSuited():
		return fmt.Sprintf("%c%d", "DBC"[k.Suit()], k.Number())
	case k.IsHonor():
		return [...]string{"East", "South", "West", "North", "Red", "Green", "White"}[k-NumSuited]
	case k >= KindFlower1 && k < KindSeason1:
		return fmt.Sprintf("Flower%d", int(k-KindFlower1)+1)
	case k >= KindSeason1 && k < NumKinds:
		return fmt.Sprintf("Season%d", int(k-KindSeason1)+1)
	}
	return fmt.Sprintf("Kind(%d)", uint8(k))
}

// Tile is one physical tile. ID is unique (0-143); Kind is its type.
type Tile struct {
	ID   uint8 `json:"id"`
	Kind Kind  `json:"kind"`
}

// NewTile builds the tile with the given physical ID (0-143).
// IDs 0-135 are play tiles (4 copies per kind, Kind = ID/4); 136-143 are the 8 bonus tiles.
func NewTile(id uint8) Tile {
	if id < NumPlayKinds*4 {
		return Tile{ID: id, Kind: Kind(id / 4)}
	}
	return Tile{ID: id, Kind: Kind(NumPlayKinds + (id - NumPlayKinds*4))}
}

func (t Tile) String() string { return fmt.Sprintf("%s#%d", t.Kind, t.ID) }
