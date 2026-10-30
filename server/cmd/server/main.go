// Command server runs the mahjong game server.
//
//	go run ./cmd/server                       # WebSocket at ws://localhost:8080/ws
//	go run ./cmd/server -web ../web/dist      # also serve the built frontend
//	                                          # (prints the LAN address to share)
package main

import (
	"flag"
	"log"
	"net"
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
	if *web != "" {
		for i, u := range lanURLs(*addr) {
			if i == 0 {
				log.Printf("  open on this network: %s", u)
			} else {
				log.Printf("  other addresses:      %s", u)
			}
		}
	}
	log.Fatal(http.ListenAndServe(*addr, mux))
}

// lanURLs lists http://<ip>:<port> for each private IPv4 address of this
// machine, so players on the same network know what to open. The address of
// the main network connection comes first.
func lanURLs(addr string) []string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil
	}
	if host != "" && host != "0.0.0.0" && host != "::" {
		return []string{"http://" + net.JoinHostPort(host, port)}
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	main := primaryIP()
	var urls []string
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipnet.IP.To4()
		if ip == nil || !ip.IsPrivate() {
			continue
		}
		u := "http://" + net.JoinHostPort(ip.String(), port)
		if ip.Equal(main) {
			urls = append([]string{u}, urls...)
		} else {
			urls = append(urls, u)
		}
	}
	return urls
}

// primaryIP is the local address the OS would use to reach the internet.
// A UDP "dial" sends no packets; it only picks a route.
func primaryIP() net.IP {
	c, err := net.Dial("udp4", "8.8.8.8:80")
	if err != nil {
		return nil
	}
	defer c.Close()
	return c.LocalAddr().(*net.UDPAddr).IP
}
