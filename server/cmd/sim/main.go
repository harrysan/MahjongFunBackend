// Command sim plays bot-only games and prints statistics.
//
//	go run ./cmd/sim                 # 1000 games
//	go run ./cmd/sim -games 5000
//	go run ./cmd/sim -v -seed 7      # print every event of one game
package main

import (
	"flag"
	"fmt"
	"os"

	"mahjong/server/internal/bot"
	"mahjong/server/internal/engine"
)

func main() {
	games := flag.Int("games", 1000, "number of games to play")
	seed := flag.Int64("seed", 1, "seed of the first game")
	verbose := flag.Bool("v", false, "print every event of the first game")
	flag.Parse()

	draws, selfWins, discardWins, totalSteps := 0, 0, 0, 0
	for i := 0; i < *games; i++ {
		s := *seed + int64(i)
		var bots [engine.NumSeats]*bot.Bot
		for k := range bots {
			bots[k] = bot.New(s*10 + int64(k))
		}
		g := engine.NewGame(s, engine.Seat(s%engine.NumSeats))
		steps := 0
		for ; !g.Ended(); steps++ {
			if steps > 5000 {
				fmt.Fprintf(os.Stderr, "game with seed %d did not finish\n", s)
				os.Exit(1)
			}
			seat := g.Pending()[0]
			act := bots[seat].Choose(g.ViewFor(seat))
			events, err := g.Apply(seat, act)
			if err != nil {
				fmt.Fprintf(os.Stderr, "seed %d: seat %d chose %+v: %v\n", s, seat, act, err)
				os.Exit(1)
			}
			if *verbose && i == 0 {
				fmt.Printf("seat %d -> action %d\n", seat, act.Type)
				for _, e := range events {
					fmt.Printf("    %-12s seat=%d %v\n", e.Type, e.Seat, e.Tiles)
				}
			}
		}
		totalSteps += steps
		res, _ := g.Result()
		switch {
		case res.Draw:
			draws++
		case res.SelfDraw:
			selfWins++
		default:
			discardWins++
		}
	}
	n := float64(*games)
	fmt.Printf("%d games played\n", *games)
	fmt.Printf("  draws (wall empty):  %5d  (%.1f%%)\n", draws, 100*float64(draws)/n)
	fmt.Printf("  self-draw wins:      %5d  (%.1f%%)\n", selfWins, 100*float64(selfWins)/n)
	fmt.Printf("  wins on a discard:   %5d  (%.1f%%)\n", discardWins, 100*float64(discardWins)/n)
	fmt.Printf("  average steps/game:  %.0f\n", float64(totalSteps)/n)
}
