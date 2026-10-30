// Package ws connects WebSocket clients to rooms: it decodes client messages
// into room calls and encodes room messages as JSON (spec section 7).
package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"mahjong/server/internal/engine"
	"mahjong/server/internal/room"
)

const (
	sendBuffer   = 256
	writeTimeout = 10 * time.Second
	// A ping every pingInterval finds connections that died without a close
	// (phone asleep, Wi-Fi gone), so the player's seat is handed to a bot.
	pingInterval = 20 * time.Second
	pingTimeout  = 10 * time.Second
)

// Handler upgrades HTTP requests to WebSocket connections.
type Handler struct {
	Rooms *room.Manager
	// OriginPatterns lists extra allowed Origin hosts (e.g. "localhost:5173").
	OriginPatterns []string
	// Logger receives connection logs. nil = slog.Default().
	Logger *slog.Logger
	// MsgRate and MsgBurst override the per-connection message limit
	// (see limit.go); 0 = the defaults. Tests that play at machine speed raise them.
	MsgRate, MsgBurst float64
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: h.OriginPatterns})
	if err != nil {
		return // Accept already wrote the HTTP error
	}
	c.SetReadLimit(maxReadBytes)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	logger := h.Logger
	if logger == nil {
		logger = slog.Default()
	}
	cl := &client{
		conn: c, ctx: ctx, cancel: cancel, send: make(chan room.Message, sendBuffer), rooms: h.Rooms, seat: -1,
		log: logger.With("remote", r.RemoteAddr), limit: newLimiter(orDefault(h.MsgRate, msgRate), orDefault(h.MsgBurst, msgBurst)),
	}
	cl.log.Debug("connection opened")
	go cl.writeLoop()
	go cl.pingLoop()
	status, reason := cl.readLoop()

	if cl.room != nil {
		cl.room.Disconnect(cl.seat, cl) // the player may come back with rejoin_room
	}
	c.Close(status, reason)
}

func orDefault(v, def float64) float64 {
	if v > 0 {
		return v
	}
	return def
}

// envelope is one client-to-server message.
type envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type client struct {
	conn   *websocket.Conn
	ctx    context.Context
	cancel context.CancelFunc
	send   chan room.Message
	rooms  *room.Manager

	log   *slog.Logger
	limit *limiter

	// Only the read loop touches these.
	room    *room.Room // set by join_room / rejoin_room
	seat    engine.Seat
	dropped int // messages in a row refused by the rate limit
}

// Send queues a message. A client too slow to keep up is disconnected.
func (c *client) Send(m room.Message) {
	select {
	case c.send <- m:
	case <-c.ctx.Done():
	default:
		c.log.Warn("send buffer full, dropping slow client")
		c.cancel()
	}
}

// Close drops the connection; the room calls it when the same player
// connected again from somewhere else.
func (c *client) Close() { c.cancel() }

func (c *client) pingLoop() {
	t := time.NewTicker(pingInterval)
	defer t.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-t.C:
			ctx, cancel := context.WithTimeout(c.ctx, pingTimeout)
			err := c.conn.Ping(ctx)
			cancel()
			if err != nil {
				c.log.Debug("ping failed", "err", err)
				c.cancel()
				return
			}
		}
	}
}

func (c *client) writeLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case m := <-c.send:
			ctx, cancel := context.WithTimeout(c.ctx, writeTimeout)
			err := wsjson.Write(ctx, c.conn, m)
			cancel()
			if err != nil {
				c.cancel()
				return
			}
		}
	}
}

// readLoop handles messages until the connection ends, and returns how to
// close it.
func (c *client) readLoop() (websocket.StatusCode, string) {
	for {
		_, b, err := c.conn.Read(c.ctx)
		if err != nil {
			// Includes messages over maxReadBytes: the library closes the
			// connection with StatusMessageTooBig.
			c.log.Debug("connection closed", "err", err)
			return websocket.StatusNormalClosure, ""
		}
		if !c.limit.allow(time.Now()) {
			c.dropped++
			if c.dropped == 1 {
				c.log.Warn("rate limited")
				c.fail(errRateLimited)
			}
			if c.dropped >= maxDropped {
				c.log.Warn("closing connection: kept flooding", "dropped", c.dropped)
				return websocket.StatusPolicyViolation, "too many messages"
			}
			continue
		}
		c.dropped = 0
		var env envelope
		if json.Unmarshal(b, &env) != nil {
			c.fail(errBadRequest)
			continue
		}
		if e := c.handle(env); e != nil {
			c.fail(e)
		}
	}
}

func (c *client) fail(e *room.Error) { c.Send(room.Message{Type: room.MsgError, Data: e}) }

var (
	errBadRequest  = &room.Error{Code: "bad_request", Message: "Pesan tidak dikenali."}
	errNotInRoom   = &room.Error{Code: "not_in_room", Message: "Masuk ke room dulu."}
	errInRoom      = &room.Error{Code: "already_in_room", Message: "Kamu sudah ada di room."}
	errRateLimited = &room.Error{Code: "rate_limited", Message: "Terlalu banyak aksi. Tunggu sebentar."}
)

func (c *client) handle(env envelope) *room.Error {
	if env.Type == "join_room" {
		var d struct {
			Room   string `json:"room"`
			Name   string `json:"name"`
			Create bool   `json:"create"` // make a new room instead of joining one
		}
		if err := decode(env.Data, &d); err != nil {
			return errBadRequest
		}
		if c.room != nil {
			return errInRoom
		}
		r, s, err := c.rooms.Join(d.Room, d.Name, d.Create, c)
		if err != nil {
			return toError(err)
		}
		c.room, c.seat = r, s
		return nil
	}

	if env.Type == "rejoin_room" {
		var d struct {
			Room  string `json:"room"`
			Token string `json:"token"`
		}
		if err := decode(env.Data, &d); err != nil {
			return errBadRequest
		}
		if c.room != nil {
			return errInRoom
		}
		r, s, err := c.rooms.Rejoin(d.Room, d.Token, c)
		if err != nil {
			return toError(err)
		}
		c.room, c.seat = r, s
		return nil
	}

	if c.room == nil {
		return errNotInRoom
	}
	switch env.Type {
	case "add_bot":
		var d struct {
			Seat engine.Seat `json:"seat"`
		}
		if decode(env.Data, &d) != nil {
			return errBadRequest
		}
		c.room.AddBot(c.seat, d.Seat)
	case "remove_bot":
		var d struct {
			Seat engine.Seat `json:"seat"`
		}
		if decode(env.Data, &d) != nil {
			return errBadRequest
		}
		c.room.RemoveBot(c.seat, d.Seat)
	case "update_settings":
		var d room.Settings
		if env.Data == nil || decode(env.Data, &d) != nil {
			return errBadRequest
		}
		c.room.UpdateSettings(c.seat, d)
	case "leave_room":
		c.room.Leave(c.seat)
		c.room, c.seat = nil, -1
	case "start_game":
		c.room.Start(c.seat)
	case "next_round":
		c.room.NextRound(c.seat)
	case "discard", "claim", "self_action":
		a, ok := parseAction(env)
		if !ok {
			return errBadRequest
		}
		c.room.Act(c.seat, a)
	default:
		return errBadRequest
	}
	return nil
}

// parseAction turns discard / claim / self_action data into an engine action.
//
//	discard     {tile_id}
//	claim       {action: "chi"|"pung"|"kong"|"hu"|"pass", tiles?: [id, id]}
//	self_action {action: "kong"|"hu", tile_id?}
func parseAction(env envelope) (engine.Action, bool) {
	var d struct {
		Action string  `json:"action"`
		TileID *uint8  `json:"tile_id"`
		Tiles  []uint8 `json:"tiles"`
	}
	if decode(env.Data, &d) != nil {
		return engine.Action{}, false
	}
	if env.Type == "discard" {
		if d.TileID == nil {
			return engine.Action{}, false
		}
		return engine.Action{Type: engine.ActionDiscard, TileID: *d.TileID}, true
	}

	t, ok := engine.ParseActionType(d.Action)
	if !ok {
		return engine.Action{}, false
	}
	a := engine.Action{Type: t}
	switch env.Type {
	case "claim":
		switch t {
		case engine.ActionChi:
			if len(d.Tiles) != 2 {
				return engine.Action{}, false
			}
			a.TileID, a.TileID2 = d.Tiles[0], d.Tiles[1]
		case engine.ActionPung, engine.ActionKong, engine.ActionHu, engine.ActionPass:
		default:
			return engine.Action{}, false
		}
	case "self_action":
		switch t {
		case engine.ActionKong:
			if d.TileID == nil {
				return engine.Action{}, false
			}
			a.TileID = *d.TileID
		case engine.ActionHu:
		default:
			return engine.Action{}, false
		}
	}
	return a, true
}

// decode reads optional message data; a missing "data" field counts as {}.
func decode(raw json.RawMessage, v any) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return json.Unmarshal(raw, v)
}

func toError(err error) *room.Error {
	var e *room.Error
	if errors.As(err, &e) {
		return e
	}
	slog.Error("unexpected room error", "err", err)
	return &room.Error{Code: "internal", Message: "Terjadi kesalahan di server."}
}
