package engine

import "math/rand"

// RollDice rolls two six-sided dice.
func RollDice(rng *rand.Rand) [2]int {
	return [2]int{rng.Intn(6) + 1, rng.Intn(6) + 1}
}

// NextDealer picks the dealer of the next round (spec 4.7): the winner is
// counted as 1 and counting goes clockwise by the dice total; whoever the last
// count lands on becomes dealer.
func NextDealer(winner Seat, dice [2]int) Seat {
	total := dice[0] + dice[1]
	return Seat(((int(winner)+total-1)%NumSeats + NumSeats) % NumSeats)
}
