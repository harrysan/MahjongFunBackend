package room

import "time"

// All timing values live here so they are easy to change (spec 4.6, 8).
// They are the defaults of a new room; the host can change them with
// update_settings (see Settings).
const (
	TurnTimeout  = 30 * time.Second
	ClaimTimeout = 10 * time.Second
	BotMinDelay  = 500 * time.Millisecond
	BotMaxDelay  = 1500 * time.Millisecond
)

// Limits for what the host may choose.
const (
	MaxBotDelay     = 10 * time.Second
	MinTurnTimeout  = 5 * time.Second
	MaxTurnTimeout  = 120 * time.Second
	MinClaimTimeout = 3 * time.Second
	MaxClaimTimeout = 60 * time.Second
)

const (
	MaxRoomNameLen   = 32
	MaxPlayerNameLen = 20
)

// Settings are the per-room options the host picks before a round.
// A timeout of 0 means no limit.
type Settings struct {
	BotDelayMinMs int `json:"bot_delay_min_ms"`
	BotDelayMaxMs int `json:"bot_delay_max_ms"`
	TurnSeconds   int `json:"turn_seconds"`
	ClaimSeconds  int `json:"claim_seconds"`
}

func DefaultSettings() Settings {
	return Settings{
		BotDelayMinMs: int(BotMinDelay / time.Millisecond),
		BotDelayMaxMs: int(BotMaxDelay / time.Millisecond),
		TurnSeconds:   int(TurnTimeout / time.Second),
		ClaimSeconds:  int(ClaimTimeout / time.Second),
	}
}

// Valid reports whether every value is inside the limits above.
func (s Settings) Valid() bool {
	maxBot := int(MaxBotDelay / time.Millisecond)
	inRange := func(sec int, lo, hi time.Duration) bool {
		return sec == 0 || (sec >= int(lo/time.Second) && sec <= int(hi/time.Second))
	}
	return s.BotDelayMinMs >= 0 && s.BotDelayMinMs <= s.BotDelayMaxMs && s.BotDelayMaxMs <= maxBot &&
		inRange(s.TurnSeconds, MinTurnTimeout, MaxTurnTimeout) &&
		inRange(s.ClaimSeconds, MinClaimTimeout, MaxClaimTimeout)
}
