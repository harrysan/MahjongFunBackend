package room

import "mahjong/server/internal/engine"

// Message is one server-to-client message: {"type": "...", "data": {...}} (spec 7).
type Message struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// Client is a connected human. Send must not block; the room calls it from
// its own goroutine. Close drops the connection (used when the same player
// connects again from somewhere else).
type Client interface {
	Send(Message)
	Close()
}

const (
	MsgRoomState   = "room_state"
	MsgGameState   = "game_state" // Data: GameState
	MsgEvent       = "event"      // Data: EventData
	MsgClaimWindow = "claim_window"
	MsgError       = "error"
)

// EventDiceRolled is sent before a new round when the previous one had a winner.
const EventDiceRolled = "dice_rolled"

type SeatInfo struct {
	Seat engine.Seat `json:"seat"`
	Kind string      `json:"kind"` // "empty", "human" or "bot"
	Name string      `json:"name"`
	// Connected is false for a human whose connection dropped; a bot plays for them.
	Connected bool `json:"connected"`
}

type RoomState struct {
	Room     string                    `json:"room"`
	Status   string                    `json:"status"` // "waiting", "playing" or "ended"
	Host     engine.Seat               `json:"host"`
	YourSeat engine.Seat               `json:"your_seat"`
	Dealer   engine.Seat               `json:"dealer"`
	Seats    [engine.NumSeats]SeatInfo `json:"seats"`
	Settings Settings                  `json:"settings"`
	// Token lets this player reconnect to their seat (rejoin_room). Keep it secret.
	Token string `json:"token,omitempty"`
}

// GameState is a player's view plus the clock of the current step.
type GameState struct {
	engine.PlayerView
	TimeLeftMs  int `json:"time_left_ms"`  // 0 when there is no time limit
	TimeLimitMs int `json:"time_limit_ms"` // full length of the current timer
}

// EventData is an engine event as one seat may see it, or a dice roll.
type EventData struct {
	Type      string          `json:"type"`
	Seat      engine.Seat     `json:"seat"`
	Tiles     []engine.Tile   `json:"tiles,omitempty"`
	Meld      engine.MeldType `json:"meld,omitempty"`
	Result    *engine.Result  `json:"result,omitempty"`     // game_end
	Dice      *[2]int         `json:"dice,omitempty"`       // dice_rolled
	NewDealer *engine.Seat    `json:"new_dealer,omitempty"` // dice_rolled
}

// ClaimWindowData tells one player which claims they may make on a discard.
type ClaimWindowData struct {
	Tile    engine.Tile     `json:"tile"`
	From    engine.Seat     `json:"from"`
	Actions []engine.Action `json:"actions"`
	// TimeLeftMs is how long the window stays open; 0 when there is no limit.
	TimeLeftMs int `json:"time_left_ms"`
}

// Error is a rejected request. Code is stable for programs, Message is shown to players.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

var (
	ErrRoomFull     = &Error{"room_full", "Room sudah penuh."}
	ErrGameStarted  = &Error{"game_started", "Permainan sudah dimulai."}
	ErrNotHost      = &Error{"not_host", "Hanya host yang boleh melakukan ini."}
	ErrBadSeat      = &Error{"bad_seat", "Kursi tidak valid."}
	ErrSeatTaken    = &Error{"seat_taken", "Kursi sudah terisi."}
	ErrRoomNotFull  = &Error{"room_not_full", "Semua kursi harus terisi dulu."}
	ErrNoGame       = &Error{"no_game", "Permainan belum dimulai."}
	ErrRoundRunning = &Error{"round_running", "Ronde masih berjalan."}
	ErrBadName      = &Error{"bad_name", "Nama room atau pemain tidak valid."}
	ErrRoomClosed   = &Error{"room_closed", "Room sudah ditutup."}
	ErrNotYourTurn  = &Error{"not_your_turn", "Bukan giliranmu."}
	ErrIllegal      = &Error{"illegal_action", "Aksi tidak diperbolehkan."}
	ErrNotInHand    = &Error{"tile_not_in_hand", "Tile tidak ada di tangan."}
	ErrGameEnded    = &Error{"game_ended", "Ronde sudah selesai."}
	ErrBadSettings  = &Error{"bad_settings", "Pengaturan tidak valid."}
	ErrRoomNotFound = &Error{"room_not_found", "Room tidak ditemukan. Periksa kodenya."}
	ErrRoomExists   = &Error{"room_exists", "Kode room sudah dipakai."}
	ErrNotBot       = &Error{"not_bot", "Kursi itu tidak berisi bot."}
	ErrBadToken     = &Error{"bad_token", "Sesi sudah tidak berlaku."}
	ErrSessionMoved = &Error{"session_moved", "Permainan ini dibuka di tab atau perangkat lain."}
	ErrServerFull   = &Error{"server_full", "Server sedang penuh. Coba lagi beberapa saat lagi."}
)

// engineError converts an engine error into a player-facing Error.
func engineError(err error) *Error {
	switch err {
	case engine.ErrNotYourTurn:
		return ErrNotYourTurn
	case engine.ErrTileNotInHand:
		return ErrNotInHand
	case engine.ErrGameEnded:
		return ErrGameEnded
	}
	return ErrIllegal
}
