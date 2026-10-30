package ws

import "time"

// Limits on what one connection may send. A real player sends a few
// messages per second at most (spam-clicking a tile included); joining a
// solo game sends about six at once.
const (
	maxReadBytes = 4096 // bigger messages close the connection
	msgRate      = 10   // messages per second, on average
	msgBurst     = 20   // messages allowed at once
	// maxDropped closes a connection that keeps sending after it was told
	// to slow down: this many messages in a row over the limit.
	maxDropped = 50
)

// limiter is a token bucket: it holds up to burst tokens, refills rate tokens
// per second, and each message takes one.
type limiter struct {
	rate, burst float64
	tokens      float64
	last        time.Time
}

func newLimiter(rate, burst float64) *limiter {
	return &limiter{rate: rate, burst: burst, tokens: burst}
}

// allow reports whether a message arriving at now is within the limit.
func (l *limiter) allow(now time.Time) bool {
	if !l.last.IsZero() {
		l.tokens = min(l.burst, l.tokens+now.Sub(l.last).Seconds()*l.rate)
	}
	l.last = now
	if l.tokens < 1 {
		return false
	}
	l.tokens--
	return true
}
