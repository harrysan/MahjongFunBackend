package engine

// Counts holds how many tiles of each play kind a hand contains. Bonus tiles are not counted.
type Counts [NumPlayKinds]uint8

// CountsOf counts the play tiles in tiles (bonus tiles are skipped).
func CountsOf(tiles []Tile) Counts {
	var c Counts
	for _, t := range tiles {
		if !t.Kind.IsBonus() {
			c[t.Kind]++
		}
	}
	return c
}

// IsWinning reports whether the concealed tiles in c, together with `melds`
// already-declared melds (chi/pung/kong, each counting as one set), form a
// complete hand: 4 sets + 1 pair. Special hands (seven pairs etc.) are not supported.
func IsWinning(c Counts, melds int) bool {
	need := 4 - melds
	if need < 0 {
		return false
	}
	total := 0
	for _, n := range c {
		total += int(n)
	}
	if total != need*3+2 {
		return false
	}
	for k := 0; k < NumPlayKinds; k++ {
		if c[k] >= 2 {
			c[k] -= 2
			ok := canMakeSets(&c, need)
			c[k] += 2
			if ok {
				return true
			}
		}
	}
	return false
}

// canMakeSets reports whether every tile left in c can be split into exactly `need` sets.
// It always works on the lowest remaining kind, trying a pung first and then a chow.
func canMakeSets(c *Counts, need int) bool {
	i := 0
	for i < NumPlayKinds && c[i] == 0 {
		i++
	}
	if i == NumPlayKinds {
		return need == 0
	}
	if need == 0 {
		return false
	}
	if c[i] >= 3 {
		c[i] -= 3
		ok := canMakeSets(c, need-1)
		c[i] += 3
		if ok {
			return true
		}
	}
	k := Kind(i)
	if k.IsSuited() && k.Number() <= 7 && c[i+1] > 0 && c[i+2] > 0 {
		c[i]--
		c[i+1]--
		c[i+2]--
		ok := canMakeSets(c, need-1)
		c[i]++
		c[i+1]++
		c[i+2]++
		if ok {
			return true
		}
	}
	return false
}

// Waits returns every kind that would complete the hand if added to c
// (c should hold 3n+1 tiles). Result is in ascending Kind order.
func Waits(c Counts, melds int) []Kind {
	var res []Kind
	for k := 0; k < NumPlayKinds; k++ {
		if c[k] >= 4 {
			continue
		}
		c[k]++
		if IsWinning(c, melds) {
			res = append(res, Kind(k))
		}
		c[k]--
	}
	return res
}
