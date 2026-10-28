// Command server runs the mahjong game server.
//
//	go run ./cmd/server                       # WebSocket at ws://localhost:8080/ws
//	go run ./cmd/server -web ../web/dist      # also serve the built frontend
package main

import (
	"flag"
	"log"
	"net/http"
	"strings"

	"mahjong/server/internal/room"
	"mahjong/server/internal/ws"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	web := flag.String("web", "", "directory with the built frontend to serve at / (optional)")
	origins := flag.String("origins", "localhost:*,127.0.0.1:*", "comma-separated Origin hosts allowed to open a WebSocket")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/ws", &ws.Handler{
		Rooms:          room.NewManager(room.Options{}),
		OriginPatterns: strings.Split(*origins, ","),
	})
	if *web != "" {
		mux.Handle("/", http.FileServer(http.Dir(*web)))
	}

	log.Printf("mahjong server listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
