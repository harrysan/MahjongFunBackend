// Command server runs the mahjong game server.
//
//	go run ./cmd/server                       # WebSocket at ws://localhost:8080/ws
//	go run ./cmd/server -web ../web/dist      # also serve the built frontend
//	                                          # (prints the LAN address to share)
//
// The PORT environment variable, set by most hosting platforms, is used when
// -addr is not given. Logs go to stderr; -log-format json suits log
// collectors, -log-level debug also shows every connection.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"mahjong/server/internal/room"
	"mahjong/server/internal/ws"
)

func main() {
	defaultAddr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		defaultAddr = ":" + p
	}
	addr := flag.String("addr", defaultAddr, "listen address (default :$PORT, else :8080)")
	web := flag.String("web", "", "directory with the built frontend to serve at / (optional)")
	origins := flag.String("origins", "localhost:*,127.0.0.1:*", "comma-separated Origin hosts allowed to open a WebSocket")
	logLevel := flag.String("log-level", "info", "debug, info, warn or error")
	logFormat := flag.String("log-format", "text", "text or json")
	flag.Parse()

	logger, err := newLogger(*logLevel, *logFormat)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	slog.SetDefault(logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/ws", &ws.Handler{
		Rooms:          room.NewManager(room.Options{}),
		OriginPatterns: strings.Split(*origins, ","),
	})
	if *web != "" {
		mux.Handle("/", staticFiles(*web))
	}

	slog.Info("mahjong server listening", "addr", *addr, "web", *web)
	if *web != "" {
		for i, u := range lanURLs(*addr) {
			if i == 0 {
				slog.Info("open on this network: " + u)
			} else {
				slog.Info("other address: " + u)
			}
		}
	}

	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server stopped", "err", err)
			os.Exit(1)
		}
	}()

	// Stop on Ctrl+C or SIGTERM (sent by Docker and hosting platforms).
	stop, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	<-stop.Done()
	slog.Info("shutting down")
	ctx, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()
	_ = srv.Shutdown(ctx) // open WebSockets are hijacked; Shutdown does not wait for them
}

// staticFiles serves the built frontend. Files under /assets/ have a content
// hash in their name, so browsers may keep them for good; everything else
// (the page, the service worker, the manifest) is checked on every load so a
// new version shows up at once.
func staticFiles(dir string) http.Handler {
	// Not in Go's built-in table; browsers want it for the PWA manifest.
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}

func newLogger(level, format string) (*slog.Logger, error) {
	var lv slog.Level
	if err := lv.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("bad -log-level %q: use debug, info, warn or error", level)
	}
	opts := &slog.HandlerOptions{Level: lv}
	switch format {
	case "text":
		return slog.New(slog.NewTextHandler(os.Stderr, opts)), nil
	case "json":
		return slog.New(slog.NewJSONHandler(os.Stderr, opts)), nil
	}
	return nil, fmt.Errorf("bad -log-format %q: use text or json", format)
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
