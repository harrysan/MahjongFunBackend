package engine

// NumSeats is the number of players at a table.
const NumSeats = 4

// Seat is a position at the table: 0 East (dealer in round 1), 1 South, 2 West, 3 North.
// Play order is clockwise: 0 -> 1 -> 2 -> 3 -> 0.
type Seat int

// Next returns the seat that plays after s.
func (s Seat) Next() Seat { return Seat((int(s) + 1) % NumSeats) }

// Prev returns the seat that plays before s.
func (s Seat) Prev() Seat { return Seat((int(s) + NumSeats - 1) % NumSeats) }

// Valid reports whether s is a real seat.
func (s Seat) Valid() bool { return s >= 0 && s < NumSeats }
